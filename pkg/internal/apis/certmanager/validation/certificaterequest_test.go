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
	"bytes"
	"encoding/pem"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cminternal "github.com/jetstack/cert-manager/pkg/internal/apis/certmanager"
	"github.com/jetstack/cert-manager/pkg/util/pki"
	utilpki "github.com/jetstack/cert-manager/pkg/util/pki"
	"github.com/jetstack/cert-manager/test/unit/gen"
)

func TestValidateCertificateRequestUpdate(t *testing.T) {
	baseCR := &cminternal.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"abc":                      "123",
				"cert-manager.io/foo":      "abc",
				"acme.cert-manager.io/bar": "123",
			},
		},
		Spec: cminternal.CertificateRequestSpec{
			Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"))),
			IssuerRef: validIssuerRef,
			Usages:    nil,
			UID:       "abc",
			Username:  "user-1",
			Groups:    []string{"group-1", "group-2"},
			Extra: map[string][]string{
				"1": {"abc", "efg"},
				"2": {"efg", "abc"},
			},
		},
	}

	tests := map[string]struct {
		oldCR, newCR *cminternal.CertificateRequest
		want         field.ErrorList
	}{
		"if CertificateRequest spec and cert-manager.io annotations change, error": {
			oldCR: baseCR.DeepCopy(),
			newCR: &cminternal.CertificateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"acme.cert-manager.io/bar": "123",
						"123":                      "abc",
					},
				},
				Spec: cminternal.CertificateRequestSpec{
					Request: mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"))),
				},
			},
			want: []*field.Error{
				field.Forbidden(field.NewPath("metadata", "annotations", "cert-manager.io/foo"), "cannot change cert-manager annotation after creation"),
				field.Forbidden(field.NewPath("spec"), "cannot change spec after creation"),
			},
		},
		"if CertificateRequest spec and acme.cert-manager.io annotations change, error": {
			oldCR: baseCR.DeepCopy(),
			newCR: &cminternal.CertificateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"cert-manager.io/foo": "abc",
						"123":                 "abc",
					},
				},
				Spec: cminternal.CertificateRequestSpec{
					Request: mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"))),
				},
			},
			want: []*field.Error{
				field.Forbidden(field.NewPath("metadata", "annotations", "acme.cert-manager.io/bar"), "cannot change cert-manager annotation after creation"),
				field.Forbidden(field.NewPath("spec"), "cannot change spec after creation"),
			},
		},
		"if CertificateRequest spec and annotations do not change, don't error": {
			oldCR: baseCR.DeepCopy(),
			newCR: baseCR.DeepCopy(),
			want:  nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateUpdateCertificateRequest(nil, test.oldCR, test.newCR)
			if !reflect.DeepEqual(err, test.want) {
				t.Errorf("got unexpected error response, exp=%v got=%v",
					test.want, err)
			}
		})
	}
}

func TestValidateCertificateRequestSpec(t *testing.T) {
	fldPath := field.NewPath("test")

	tests := []struct {
		name   string
		crSpec *cminternal.CertificateRequestSpec
		want   field.ErrorList
	}{
		{
			name: "Test csr with no usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"))),
				IssuerRef: validIssuerRef,
				Usages:    nil,
			},
			want: []*field.Error{},
		},
		{
			name: "Test csr with double signature usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageSigning, cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageSigning, cminternal.UsageKeyEncipherment},
			},
			want: []*field.Error{},
		},
		{
			name: "Test csr with double extended usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageServerAuth, cmapi.UsageClientAuth))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageSigning, cminternal.UsageKeyEncipherment, cminternal.UsageServerAuth, cminternal.UsageClientAuth},
			},
			want: []*field.Error{},
		},
		{
			name: "Test csr with reordered usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageServerAuth, cmapi.UsageClientAuth))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageServerAuth, cminternal.UsageClientAuth, cminternal.UsageKeyEncipherment, cminternal.UsageDigitalSignature},
			},
			want: []*field.Error{},
		},
		{
			name: "Test csr that is CA with usages set",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageAny, cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageCertSign), gen.SetCertificateIsCA(true))),
				IssuerRef: validIssuerRef,
				IsCA:      true,
				Usages:    []cminternal.KeyUsage{cminternal.UsageAny, cminternal.UsageDigitalSignature, cminternal.UsageKeyEncipherment, cminternal.UsageCertSign},
			},
			want: []*field.Error{},
		},
		{
			name: "Test csr that is CA but no cert sign in usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageAny, cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageClientAuth, cmapi.UsageServerAuth), gen.SetCertificateIsCA(true))),
				IssuerRef: validIssuerRef,
				IsCA:      true,
				Usages:    []cminternal.KeyUsage{cminternal.UsageAny, cminternal.UsageDigitalSignature, cminternal.UsageKeyEncipherment, cminternal.UsageClientAuth, cminternal.UsageServerAuth},
			},
			want: []*field.Error{},
		},
		{
			name: "Error on csr not having all usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageServerAuth))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageSigning, cminternal.UsageKeyEncipherment, cminternal.UsageServerAuth, cminternal.UsageClientAuth},
			},
			want: []*field.Error{
				field.Invalid(fldPath.Child("request"), nil, "csr key usages do not match specified usages, these should match if both are set: [[]certmanager.KeyUsage[3] != []certmanager.KeyUsage[4]]"),
			},
		},
		{
			name: "Error on cr not having all usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageServerAuth, cmapi.UsageClientAuth))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageSigning, cminternal.UsageKeyEncipherment},
			},
			want: []*field.Error{
				field.Invalid(fldPath.Child("request"), nil, "csr key usages do not match specified usages, these should match if both are set: [[]certmanager.KeyUsage[4] != []certmanager.KeyUsage[2]]"),
			},
		},
		{
			name: "Error on cr not having all usages",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageDigitalSignature, cmapi.UsageKeyEncipherment, cmapi.UsageServerAuth, cmapi.UsageClientAuth))),
				IssuerRef: validIssuerRef,
				Usages:    []cminternal.KeyUsage{cminternal.UsageAny, cminternal.UsageSigning},
			},
			want: []*field.Error{
				field.Invalid(fldPath.Child("request"), nil, "csr key usages do not match specified usages, these should match if both are set: [[]certmanager.KeyUsage[4] != []certmanager.KeyUsage[2]]"),
			},
		},
		{
			name: "Test csr with any, signing, digital signature, key encipherment, server and client auth",
			crSpec: &cminternal.CertificateRequestSpec{
				Request:   mustGenerateCSR(t, gen.Certificate("test", gen.SetCertificateDNSNames("example.com"), gen.SetCertificateKeyUsages(cmapi.UsageAny, cmapi.UsageSigning, cmapi.UsageKeyEncipherment, cmapi.UsageClientAuth, cmapi.UsageServerAuth), gen.SetCertificateIsCA(true))),
				IssuerRef: validIssuerRef,
				IsCA:      true,
				Usages:    []cminternal.KeyUsage{cminternal.UsageAny, cminternal.UsageSigning, cminternal.UsageKeyEncipherment, cminternal.UsageClientAuth, cminternal.UsageServerAuth},
			},
			want: []*field.Error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateCertificateRequestSpec(tt.crSpec, field.NewPath("test"), true)
			for i := range got {
				// filter out the value so it does not print the full CSR in tests
				got[i].BadValue = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateCertificateRequestSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustGenerateCSR(t *testing.T, crt *cmapi.Certificate) []byte {
	// Create a new private key
	pk, err := utilpki.GenerateRSAPrivateKey(2048)
	if err != nil {
		t.Fatal(err)
	}

	x509CSR, err := pki.GenerateCSR(crt)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := pki.EncodeCSR(x509CSR, pk)
	if err != nil {
		t.Fatal(err)
	}

	csrPEM := bytes.NewBuffer([]byte{})
	err = pem.Encode(csrPEM, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if err != nil {
		t.Fatal(err)
	}

	return csrPEM.Bytes()
}

func Test_patchDuplicateKeyUsage(t *testing.T) {
	tests := []struct {
		name   string
		usages []cminternal.KeyUsage
		want   []cminternal.KeyUsage
	}{
		{
			name:   "Test single KU",
			usages: []cminternal.KeyUsage{cminternal.UsageKeyEncipherment},
			want:   []cminternal.KeyUsage{cminternal.UsageKeyEncipherment},
		},
		{
			name:   "Test UsageSigning",
			usages: []cminternal.KeyUsage{cminternal.UsageSigning},
			want:   []cminternal.KeyUsage{cminternal.UsageDigitalSignature},
		},
		{
			name:   "Test multiple KU",
			usages: []cminternal.KeyUsage{cminternal.UsageDigitalSignature, cminternal.UsageServerAuth, cminternal.UsageClientAuth},
			want:   []cminternal.KeyUsage{cminternal.UsageDigitalSignature, cminternal.UsageServerAuth, cminternal.UsageClientAuth},
		},
		{
			name:   "Test double signing",
			usages: []cminternal.KeyUsage{cminternal.UsageSigning, cminternal.UsageDigitalSignature},
			want:   []cminternal.KeyUsage{cminternal.UsageDigitalSignature},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := patchDuplicateKeyUsage(tt.usages); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("patchDuplicateKeyUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}
