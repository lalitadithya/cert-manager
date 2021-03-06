/*
Copyright 2020 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"reflect"
	"strings"

	"github.com/kr/pretty"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/jetstack/cert-manager/pkg/apis/acme"
	"github.com/jetstack/cert-manager/pkg/apis/certmanager"
	cmapi "github.com/jetstack/cert-manager/pkg/internal/apis/certmanager"
	"github.com/jetstack/cert-manager/pkg/util"
	"github.com/jetstack/cert-manager/pkg/util/pki"
)

var defaultInternalKeyUsages = []cmapi.KeyUsage{cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment}

func ValidateCertificateRequest(_ *admissionv1.AdmissionRequest, obj runtime.Object) field.ErrorList {
	cr := obj.(*cmapi.CertificateRequest)
	allErrs := ValidateCertificateRequestSpec(&cr.Spec, field.NewPath("spec"), true)
	return allErrs
}

func ValidateUpdateCertificateRequest(_ *admissionv1.AdmissionRequest, oldObj, newObj runtime.Object) field.ErrorList {
	oldCR, newCR := oldObj.(*cmapi.CertificateRequest), newObj.(*cmapi.CertificateRequest)

	var el field.ErrorList

	// Enforce that no cert-manager annotations may be modified after creation.
	// This is to prevent changing the request during processing resulting in
	// undefined behaviour, and breaking the concept of requests being made by a
	// single user.
	annotationField := field.NewPath("metadata", "annotations")
	el = append(el, validateCertificateRequestAnnotations(oldCR, newCR, annotationField)...)
	el = append(el, validateCertificateRequestAnnotations(newCR, oldCR, annotationField)...)

	if !reflect.DeepEqual(oldCR.Spec, newCR.Spec) {
		el = append(el, field.Forbidden(field.NewPath("spec"), "cannot change spec after creation"))
	}

	return el
}

func validateCertificateRequestAnnotations(objA, objB *cmapi.CertificateRequest, fieldPath *field.Path) field.ErrorList {
	var el field.ErrorList
	for k, v := range objA.Annotations {
		if strings.HasPrefix(k, certmanager.GroupName) ||
			strings.HasPrefix(k, acme.GroupName) {
			if vnew, ok := objB.Annotations[k]; !ok || v != vnew {
				el = append(el, field.Forbidden(fieldPath.Child(k), "cannot change cert-manager annotation after creation"))
			}
		}
	}

	return el
}

func ValidateCertificateRequestSpec(crSpec *cmapi.CertificateRequestSpec, fldPath *field.Path, validateCSRContent bool) field.ErrorList {
	el := field.ErrorList{}

	el = append(el, validateIssuerRef(crSpec.IssuerRef, fldPath)...)

	if len(crSpec.Request) == 0 {
		el = append(el, field.Required(fldPath.Child("request"), "must be specified"))
	} else {
		csr, err := pki.DecodeX509CertificateRequestBytes(crSpec.Request)
		if err != nil {
			el = append(el, field.Invalid(fldPath.Child("request"), crSpec.Request, fmt.Sprintf("failed to decode csr: %s", err)))
		} else {
			// only compare usages if set on CR and in the CSR
			if len(crSpec.Usages) > 0 && len(csr.Extensions) > 0 && validateCSRContent && !reflect.DeepEqual(crSpec.Usages, defaultInternalKeyUsages) {
				if crSpec.IsCA {
					crSpec.Usages = ensureCertSignIsSet(crSpec.Usages)
				}
				csrUsages, err := getCSRKeyUsage(crSpec, fldPath, csr, el)
				if len(err) > 0 {
					el = append(el, err...)
				} else if len(csrUsages) > 0 && !isUsageEqual(csrUsages, crSpec.Usages) && !isUsageEqual(csrUsages, defaultInternalKeyUsages) {
					el = append(el, field.Invalid(fldPath.Child("request"), crSpec.Request, fmt.Sprintf("csr key usages do not match specified usages, these should match if both are set: %s", pretty.Diff(patchDuplicateKeyUsage(csrUsages), patchDuplicateKeyUsage(crSpec.Usages)))))
				}
			}
		}
	}

	return el
}

func getCSRKeyUsage(crSpec *cmapi.CertificateRequestSpec, fldPath *field.Path, csr *x509.CertificateRequest, el field.ErrorList) ([]cmapi.KeyUsage, field.ErrorList) {
	var ekus []x509.ExtKeyUsage
	var ku x509.KeyUsage

	for _, extension := range csr.Extensions {
		if extension.Id.String() == asn1.ObjectIdentifier(pki.OIDExtensionExtendedKeyUsage).String() {
			var asn1ExtendedUsages []asn1.ObjectIdentifier
			_, err := asn1.Unmarshal(extension.Value, &asn1ExtendedUsages)
			if err != nil {
				el = append(el, field.Invalid(fldPath.Child("request"), crSpec.Request, fmt.Sprintf("failed to decode csr extended usages: %s", err)))
			} else {
				for _, asnExtUsage := range asn1ExtendedUsages {
					eku, ok := pki.ExtKeyUsageFromOID(asnExtUsage)
					if ok {
						ekus = append(ekus, eku)
					}
				}
			}
		}
		if extension.Id.String() == asn1.ObjectIdentifier(pki.OIDExtensionKeyUsage).String() {
			// RFC 5280, 4.2.1.3
			var asn1bits asn1.BitString
			_, err := asn1.Unmarshal(extension.Value, &asn1bits)
			if err != nil {
				el = append(el, field.Invalid(fldPath.Child("request"), crSpec.Request, fmt.Sprintf("failed to decode csr usages: %s", err)))
			} else {
				var usage int
				for i := 0; i < 9; i++ {
					if asn1bits.At(i) != 0 {
						usage |= 1 << uint(i)
					}
				}
				ku = x509.KeyUsage(usage)
			}
		}
	}

	// convert usages to the internal API
	var out []cmapi.KeyUsage
	for _, usage := range pki.BuildCertManagerKeyUsages(ku, ekus) {
		out = append(out, cmapi.KeyUsage(usage))
	}
	return out, el
}

func patchDuplicateKeyUsage(usages []cmapi.KeyUsage) []cmapi.KeyUsage {
	// usage signing and digital signature are the same key use in x509
	// we should patch this for proper validation

	newUsages := []cmapi.KeyUsage(nil)
	hasUsageSigning := false
	for _, usage := range usages {
		if (usage == cmapi.UsageSigning || usage == cmapi.UsageDigitalSignature) && !hasUsageSigning {
			newUsages = append(newUsages, cmapi.UsageDigitalSignature)
			// prevent having 2 UsageDigitalSignature in the slice
			hasUsageSigning = true
		} else if usage != cmapi.UsageSigning && usage != cmapi.UsageDigitalSignature {
			newUsages = append(newUsages, usage)
		}
	}

	return newUsages
}

func isUsageEqual(a, b []cmapi.KeyUsage) bool {
	a = patchDuplicateKeyUsage(a)
	b = patchDuplicateKeyUsage(b)

	var aStrings, bStrings []string

	for _, usage := range a {
		aStrings = append(aStrings, string(usage))
	}

	for _, usage := range b {
		bStrings = append(bStrings, string(usage))
	}

	return util.EqualUnsorted(aStrings, bStrings)
}

// ensureCertSignIsSet adds UsageCertSign in case it is not set
// TODO: add a mutating webhook to make sure this is always set
// when isCA is true.
func ensureCertSignIsSet(list []cmapi.KeyUsage) []cmapi.KeyUsage {
	for _, usage := range list {
		if usage == cmapi.UsageCertSign {
			return list
		}
	}

	return append(list, cmapi.UsageCertSign)
}
