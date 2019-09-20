/*
Copyright 2019 The Jetstack cert-manager contributors.

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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
)

var (
	validCloudDNSProvider = v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
		ServiceAccount: validSecretKeyRef,
		Project:        "valid",
	}
	validSecretKeyRef = v1alpha2.SecretKeySelector{
		LocalObjectReference: v1alpha2.LocalObjectReference{
			Name: "valid",
		},
		Key: "validkey",
	}
	validCloudflareProvider = v1alpha2.ACMEIssuerDNS01ProviderCloudflare{
		APIKey: validSecretKeyRef,
		Email:  "valid",
	}
	validACMEIssuer = v1alpha2.ACMEIssuer{
		Email:      "valid-email",
		Server:     "valid-server",
		PrivateKey: validSecretKeyRef,
	}
	validVaultIssuer = v1alpha2.VaultIssuer{
		Auth: v1alpha2.VaultAuth{
			TokenSecretRef: validSecretKeyRef,
		},
		Server: "something",
		Path:   "a/b/c",
	}
)

func TestValidateVaultIssuerConfig(t *testing.T) {
	fldPath := field.NewPath("")
	scenarios := map[string]struct {
		spec *v1alpha2.VaultIssuer
		errs []*field.Error
	}{
		"valid vault issuer": {
			spec: &validVaultIssuer,
		},
		"vault issuer with missing fields": {
			spec: &v1alpha2.VaultIssuer{},
			errs: []*field.Error{
				field.Required(fldPath.Child("server"), ""),
				field.Required(fldPath.Child("path"), ""),
			},
		},
		"vault issuer with invalid fields": {
			spec: &v1alpha2.VaultIssuer{
				Server:   "something",
				Path:     "a/b/c",
				CABundle: []byte("invalid"),
			},
			errs: []*field.Error{
				field.Invalid(fldPath.Child("caBundle"), "", "Specified CA bundle is invalid"),
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateVaultIssuerConfig(s.spec, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}

func TestValidateACMEIssuerConfig(t *testing.T) {
	fldPath := field.NewPath("")
	scenarios := map[string]struct {
		spec *v1alpha2.ACMEIssuer
		errs []*field.Error
	}{
		"valid acme issuer": {
			spec: &validACMEIssuer,
		},
		"acme issuer with missing fields": {
			spec: &v1alpha2.ACMEIssuer{},
			errs: []*field.Error{
				field.Required(fldPath.Child("privateKeySecretRef", "name"), "private key secret name is a required field"),
				field.Required(fldPath.Child("server"), "acme server URL is a required field"),
			},
		},
		"acme solver without any config": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{},
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("solvers").Index(0), "no solver type configured"),
			},
		},
		"acme solver with valid dns01 config": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						DNS01: &v1alpha2.ACMEChallengeSolverDNS01{
							CloudDNS: &validCloudDNSProvider,
						},
					},
				},
			},
		},
		"acme solver with missing http01 config type": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{},
					},
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("solvers").Index(0).Child("http01"), "no HTTP01 solver type configured"),
			},
		},
		"acme solver with valid http01 config": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{
							Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{},
						},
					},
				},
			},
		},
		"acme issue with valid pod template ObjectMeta attributes": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{
							Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
								PodTemplate: &v1alpha2.ACMEChallengeSolverHTTP01IngressPodTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"valid_to_contain": "labels",
										},
										Annotations: map[string]string{
											"valid_to_contain": "annotations",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"acme issue with invalid pod template ObjectMeta attributes": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{
							Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
								PodTemplate: &v1alpha2.ACMEChallengeSolverHTTP01IngressPodTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"valid_to_contain": "annotations",
										},
										GenerateName: "unable-to-change-generateName",
										Name:         "unable-to-change-name",
									},
								},
							},
						},
					},
				},
			},
			errs: []*field.Error{
				field.Invalid(fldPath.Child("solvers").Index(0).Child("http01", "ingress", "podTemplate", "metadata"),
					"", "only labels and annotations may be set on podTemplate metadata"),
			},
		},
		"acme issue with valid pod template PodSpec attributes": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{
							Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
								PodTemplate: &v1alpha2.ACMEChallengeSolverHTTP01IngressPodTemplate{
									Spec: v1alpha2.ACMEChallengeSolverHTTP01IngressPodSpec{
										NodeSelector: map[string]string{
											"valid_to_contain": "nodeSelector",
										},
										Tolerations: []corev1.Toleration{
											{
												Key:      "valid_key",
												Operator: "Exists",
												Effect:   "NoSchedule",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"acme issue with valid pod template ObjectMeta and PodSpec attributes": {
			spec: &v1alpha2.ACMEIssuer{
				Email:      "valid-email",
				Server:     "valid-server",
				PrivateKey: validSecretKeyRef,
				Solvers: []v1alpha2.ACMEChallengeSolver{
					{
						HTTP01: &v1alpha2.ACMEChallengeSolverHTTP01{
							Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
								PodTemplate: &v1alpha2.ACMEChallengeSolverHTTP01IngressPodTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"valid_to_contain": "labels",
										},
									},
									Spec: v1alpha2.ACMEChallengeSolverHTTP01IngressPodSpec{
										NodeSelector: map[string]string{
											"valid_to_contain": "nodeSelector",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateACMEIssuerConfig(s.spec, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}

func TestValidateIssuerSpec(t *testing.T) {
	fldPath := field.NewPath("")
	scenarios := map[string]struct {
		spec *v1alpha2.IssuerSpec
		errs []*field.Error
	}{
		"valid ca issuer": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					CA: &v1alpha2.CAIssuer{
						SecretName: "valid",
					},
				},
			},
		},
		"ca issuer without secret name specified": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					CA: &v1alpha2.CAIssuer{},
				},
			},
			errs: []*field.Error{field.Required(fldPath.Child("ca", "secretName"), "")},
		},
		"valid self signed issuer": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					SelfSigned: &v1alpha2.SelfSignedIssuer{},
				},
			},
		},
		"valid acme issuer": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					ACME: &validACMEIssuer,
				},
			},
		},
		"valid vault issuer": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					Vault: &validVaultIssuer,
				},
			},
		},
		"missing issuer config": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{},
			},
			errs: []*field.Error{
				field.Required(fldPath, "at least one issuer must be configured"),
			},
		},
		"multiple issuers configured": {
			spec: &v1alpha2.IssuerSpec{
				IssuerConfig: v1alpha2.IssuerConfig{
					SelfSigned: &v1alpha2.SelfSignedIssuer{},
					CA: &v1alpha2.CAIssuer{
						SecretName: "valid",
					},
				},
			},
			errs: []*field.Error{
				field.Forbidden(fldPath.Child("selfSigned"), "may not specify more than one issuer type"),
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateIssuerSpec(s.spec, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}

func TestValidateACMEIssuerHTTP01Config(t *testing.T) {
	fldPath := field.NewPath("")
	scenarios := map[string]struct {
		isExpectedFailure bool
		cfg               *v1alpha2.ACMEChallengeSolverHTTP01
		errs              []*field.Error
	}{
		"ingress field specified": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{Name: "abc"},
			},
		},
		"ingress class field specified": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{Class: strPtr("abc")},
			},
		},
		"neither field specified": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{},
			},
		},
		"no solver config type specified": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{},
			errs: []*field.Error{
				field.Required(fldPath, "no HTTP01 solver type configured"),
			},
		},
		"both fields specified": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
					Name:  "abc",
					Class: strPtr("abc"),
				},
			},
			errs: []*field.Error{
				field.Forbidden(fldPath.Child("ingress"), "only one of 'name' or 'class' should be specified"),
			},
		},
		"acme issuer with valid http01 service config serviceType ClusterIP": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
					ServiceType: corev1.ServiceType("ClusterIP"),
				},
			},
		},
		"acme issuer with valid http01 service config serviceType NodePort": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
					ServiceType: corev1.ServiceType("NodePort"),
				},
			},
		},
		"acme issuer with valid http01 service config serviceType (empty string)": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
					ServiceType: corev1.ServiceType(""),
				},
			},
		},
		"acme issuer with invalid http01 service config": {
			cfg: &v1alpha2.ACMEChallengeSolverHTTP01{
				Ingress: &v1alpha2.ACMEChallengeSolverHTTP01Ingress{
					ServiceType: corev1.ServiceType("InvalidServiceType"),
				},
			},
			errs: []*field.Error{
				field.Invalid(fldPath.Child("ingress", "serviceType"), corev1.ServiceType("InvalidServiceType"), `must be empty, "ClusterIP" or "NodePort"`),
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateACMEIssuerChallengeSolverHTTP01Config(s.cfg, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}

func TestValidateACMEIssuerDNS01Config(t *testing.T) {
	fldPath := field.NewPath("test")
	scenarios := map[string]struct {
		cfg  *v1alpha2.ACMEChallengeSolverDNS01
		errs []*field.Error
	}{
		"missing clouddns project": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				CloudDNS: &v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
					ServiceAccount: validSecretKeyRef,
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("clouddns", "project"), ""),
			},
		},
		"missing clouddns service account key": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				CloudDNS: &v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
					Project: "valid",
					ServiceAccount: v1alpha2.SecretKeySelector{
						LocalObjectReference: v1alpha2.LocalObjectReference{Name: "something"},
						Key:                  "",
					},
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("clouddns", "serviceAccountSecretRef", "key"), "secret key is required"),
			},
		},
		"missing clouddns service account name": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				CloudDNS: &v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
					Project: "valid",
					ServiceAccount: v1alpha2.SecretKeySelector{
						LocalObjectReference: v1alpha2.LocalObjectReference{Name: ""},
						Key:                  "something",
					},
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("clouddns", "serviceAccountSecretRef", "name"), "secret name is required"),
			},
		},
		"clouddns serviceAccount field not set should be allowed for ambient auth": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				CloudDNS: &v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
					Project: "valid",
				},
			},
		},
		"missing cloudflare token": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				Cloudflare: &v1alpha2.ACMEIssuerDNS01ProviderCloudflare{
					Email: "valid",
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("cloudflare", "apiKeySecretRef", "name"), "secret name is required"),
				field.Required(fldPath.Child("cloudflare", "apiKeySecretRef", "key"), "secret key is required"),
			},
		},
		"missing cloudflare email": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				Cloudflare: &v1alpha2.ACMEIssuerDNS01ProviderCloudflare{
					APIKey: validSecretKeyRef,
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("cloudflare", "email"), ""),
			},
		},
		"missing route53 region": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				Route53: &v1alpha2.ACMEIssuerDNS01ProviderRoute53{},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("route53", "region"), ""),
			},
		},
		"missing provider config": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{},
			errs: []*field.Error{
				field.Required(fldPath, "no DNS01 provider configured"),
			},
		},
		"missing azuredns config": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				AzureDNS: &v1alpha2.ACMEIssuerDNS01ProviderAzureDNS{},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("azuredns", "clientSecretSecretRef", "name"), "secret name is required"),
				field.Required(fldPath.Child("azuredns", "clientSecretSecretRef", "key"), "secret key is required"),
				field.Required(fldPath.Child("azuredns", "clientID"), ""),
				field.Required(fldPath.Child("azuredns", "subscriptionID"), ""),
				field.Required(fldPath.Child("azuredns", "tenantID"), ""),
				field.Required(fldPath.Child("azuredns", "resourceGroupName"), ""),
			},
		},
		"invalid azuredns environment": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				AzureDNS: &v1alpha2.ACMEIssuerDNS01ProviderAzureDNS{
					Environment: "an env",
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("azuredns", "clientSecretSecretRef", "name"), "secret name is required"),
				field.Required(fldPath.Child("azuredns", "clientSecretSecretRef", "key"), "secret key is required"),
				field.Required(fldPath.Child("azuredns", "clientID"), ""),
				field.Required(fldPath.Child("azuredns", "subscriptionID"), ""),
				field.Required(fldPath.Child("azuredns", "tenantID"), ""),
				field.Required(fldPath.Child("azuredns", "resourceGroupName"), ""),
				field.Invalid(fldPath.Child("azuredns", "environment"), v1alpha2.AzureDNSEnvironment("an env"),
					"must be either empty or one of AzurePublicCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernmentCloud"),
			},
		},
		"missing akamai config": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				Akamai: &v1alpha2.ACMEIssuerDNS01ProviderAkamai{},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("akamai", "accessToken", "name"), "secret name is required"),
				field.Required(fldPath.Child("akamai", "accessToken", "key"), "secret key is required"),
				field.Required(fldPath.Child("akamai", "clientSecret", "name"), "secret name is required"),
				field.Required(fldPath.Child("akamai", "clientSecret", "key"), "secret key is required"),
				field.Required(fldPath.Child("akamai", "clientToken", "name"), "secret name is required"),
				field.Required(fldPath.Child("akamai", "clientToken", "key"), "secret key is required"),
				field.Required(fldPath.Child("akamai", "serviceConsumerDomain"), ""),
			},
		},
		"valid akamai config": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				Akamai: &v1alpha2.ACMEIssuerDNS01ProviderAkamai{
					AccessToken:           validSecretKeyRef,
					ClientSecret:          validSecretKeyRef,
					ClientToken:           validSecretKeyRef,
					ServiceConsumerDomain: "abc",
				},
			},
			errs: []*field.Error{},
		},
		"valid rfc2136 config": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver: "127.0.0.1",
				},
			},
			errs: []*field.Error{},
		},
		"missing rfc2136 required field": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("rfc2136", "nameserver"), ""),
			},
		},
		"rfc2136 provider invalid nameserver": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver: "dns.example.com",
				},
			},
			errs: []*field.Error{
				field.Invalid(fldPath.Child("rfc2136", "nameserver"), "", "Nameserver invalid. Check the documentation for details."),
			},
		},
		"rfc2136 provider using case-camel in algorithm": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver:    "127.0.0.1",
					TSIGAlgorithm: "HmAcMd5",
				},
			},
			errs: []*field.Error{},
		},
		"rfc2136 provider using unsupported algorithm": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver:    "127.0.0.1",
					TSIGAlgorithm: "HAMMOCK",
				},
			},
			errs: []*field.Error{
				field.NotSupported(fldPath.Child("rfc2136", "tsigAlgorithm"), "", supportedTSIGAlgorithms),
			},
		},
		"rfc2136 provider TSIGKeyName provided but no TSIGSecret": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver:  "127.0.0.1",
					TSIGKeyName: "some-name",
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("rfc2136", "tsigSecretSecretRef", "name"), "secret name is required"),
				field.Required(fldPath.Child("rfc2136", "tsigSecretSecretRef", "key"), "secret key is required"),
			},
		},
		"rfc2136 provider TSIGSecret provided but no TSIGKeyName": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				RFC2136: &v1alpha2.ACMEIssuerDNS01ProviderRFC2136{
					Nameserver: "127.0.0.1",
					TSIGSecret: validSecretKeyRef,
				},
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("rfc2136", "tsigKeyName"), ""),
			},
		},
		"multiple providers configured": {
			cfg: &v1alpha2.ACMEChallengeSolverDNS01{
				CloudDNS: &v1alpha2.ACMEIssuerDNS01ProviderCloudDNS{
					Project: "something",
				},
				Cloudflare: &v1alpha2.ACMEIssuerDNS01ProviderCloudflare{},
			},
			errs: []*field.Error{
				field.Forbidden(fldPath.Child("cloudflare"), "may not specify more than one provider type"),
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateACMEChallengeSolverDNS01(s.cfg, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}

func TestValidateSecretKeySelector(t *testing.T) {
	validName := v1alpha2.LocalObjectReference{Name: "name"}
	validKey := "key"
	// invalidName := v1alpha2.LocalObjectReference{"-name-"}
	// invalidKey := "-key-"
	fldPath := field.NewPath("")
	scenarios := map[string]struct {
		isExpectedFailure bool
		selector          *v1alpha2.SecretKeySelector
		errs              []*field.Error
	}{
		"valid selector": {
			selector: &v1alpha2.SecretKeySelector{
				LocalObjectReference: validName,
				Key:                  validKey,
			},
		},
		// "invalid name": {
		// 	isExpectedFailure: true,
		// 	selector: &v1alpha2.SecretKeySelector{
		// 		LocalObjectReference: invalidName,
		// 		Key:                  validKey,
		// 	},
		// },
		// "invalid key": {
		// 	isExpectedFailure: true,
		// 	selector: &v1alpha2.SecretKeySelector{
		// 		LocalObjectReference: validName,
		// 		Key:                  invalidKey,
		// 	},
		// },
		"missing name": {
			selector: &v1alpha2.SecretKeySelector{
				Key: validKey,
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("name"), "secret name is required"),
			},
		},
		"missing key": {
			selector: &v1alpha2.SecretKeySelector{
				LocalObjectReference: validName,
			},
			errs: []*field.Error{
				field.Required(fldPath.Child("key"), "secret key is required"),
			},
		},
	}
	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			errs := ValidateSecretKeySelector(s.selector, fldPath)
			if len(errs) != len(s.errs) {
				t.Errorf("Expected %v but got %v", s.errs, errs)
				return
			}
			for i, e := range errs {
				expectedErr := s.errs[i]
				if !reflect.DeepEqual(e, expectedErr) {
					t.Errorf("Expected %v but got %v", expectedErr, e)
				}
			}
		})
	}
}