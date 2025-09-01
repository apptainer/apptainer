// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// Copyright (c) 2020-2022, ICS-FORTH.  All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package signature

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ocsp"
)

/*
Online Certificate Status Protocol (OCSP)

OCSP responder is used to provide real-time verification of the revocation status of an X.509 certificate.
RFC: https://www.rfc-editor.org/rfc/rfc6960
*/

const (
	// PKIXOCSPNoCheck refers to the Revocation Checking of an Authorized Responder.
	// More more info check https://oidref.com/1.3.6.1.5.5.7.48.1.5
	PKIXOCSPNoCheck = "1.3.6.1.5.5.7.48.1.5"
)

var errOCSP = errors.New("OCSP verification has failed")

func OCSPVerify(chain ...*x509.Certificate) error {
	// use the pool as an index for certificate issuers.
	// fixme: we can drop this lookup if we assume that certificate N is always signed by certificate N+1.
	pool := map[string]*x509.Certificate{}

	for _, cert := range chain {
		pool[string(cert.SubjectKeyId)] = cert
	}

	// recursively validate the certificate chain
	for _, cert := range chain {
		if err := revocationCheck(cert, pool); err != nil {
			sylog.Warningf("OCSP verification has failed. Err: %s", err)
			return errOCSP
		}
	}

	return nil
}

func revocationCheck(cert *x509.Certificate, pool map[string]*x509.Certificate) error {
	if len(cert.AuthorityKeyId) == 0 || string(cert.SubjectKeyId) == string(cert.AuthorityKeyId) {
		sylog.Infof("skip self-signed certificate (%s)", cert.Subject.String())

		return nil
	}

	// Get the CA who issued the certificate in question.
	issuer, exists := pool[string(cert.AuthorityKeyId)]
	if !exists {
		return fmt.Errorf("cannot find issuer '%s'", cert.Issuer)
	}

	sylog.Infof("Validate: cert:%s  issuer:%s", cert.Subject.CommonName, issuer.Subject.CommonName)

	// Ask OCSP for the validity of signer's certificate.
	ocspCertificate, err := checkOCSPResponse(cert, issuer)
	if err != nil {
		return fmt.Errorf("OCSP Query err: %w", err)
	}

	// 4.2.2.2  Authorized Responders
	if ocspCertificate != nil {
		// MUST reject the response if the certificate required to validate the signature on the
		// response fails to meet at least one of the following criteria:
		//   1. Matches a local configuration of OCSP signing authority for the
		//   certificate in question; or
		//
		//   2. Is the certificate of the CA that issued the certificate in
		//   question; or
		//
		//   3. Includes a value of id-ad-ocspSigning in an ExtendedKeyUsage
		//   extension and is issued by the CA that issued the certificate in
		//   question.
		isCase1 := func() bool {
			// Systems MAY provide a means of locally configuring one
			// or more OCSP signing authorities, and specifying the set of CAs for
			// which each signing authority is trusted.
			//
			// This is not our case so we always return false.
			return false
		}

		isCase2 := func() bool {
			return string(ocspCertificate.SubjectKeyId) == string(cert.AuthorityKeyId)
		}

		isCase3 := func() bool {
			for _, usage := range cert.ExtKeyUsage {
				if usage == x509.ExtKeyUsageOCSPSigning {
					if string(ocspCertificate.AuthorityKeyId) == string(cert.AuthorityKeyId) {
						return true
					}
				}
			}
			return false
		}

		if !isCase1() && !isCase2() && !isCase3() {
			return fmt.Errorf("ocsp response is rejected")
		}

		// 4.2.2.2.1  Revocation Checking of an Authorized Responder

		//  A CA may specify that an OCSP client can trust a responder for the
		//  lifetime of the responder's certificate. The CA does so by including
		//  the extension id-pkix-ocsp-nocheck.
		for _, extension := range cert.Extensions {
			if extension.Id.String() == PKIXOCSPNoCheck {
				goto skipOCSPVerification
			}
		}

		// A CA may specify how the responder's certificate be checked for
		// revocation. This can be done using CRL Distribution Points if the
		// check should be done using CRLs or CRL Distribution Points, or
		// Authority Information Access if the check should be done in some
		// other way. Details for specifying either of these two mechanisms are
		// available in [RFC2459].
		if err := revocationCheck(ocspCertificate, pool); err != nil {
			return fmt.Errorf("cannot verify OCSP server's certificate. err: %w", err)
		}

		// A CA may choose not to specify any method of revocation checking
		// for the responder's certificate, in which case, it would be up to the
		// OCSP client's local security policy to decide whether that
		// certificate should be checked for revocation or not.
		// -- Our current policy is to pass validation --
	skipOCSPVerification:
	}

	return nil
}

// checkOCSPResponse submit a revocation check request to the OCSP responder.
// If the certificate is ok to use, it returns nil.
// If the function cannot perform the check, or if the certificate is not ok for use (revoked or unknown), it returns
// with an error.
func checkOCSPResponse(cert, issuer *x509.Certificate) (needsValidation *x509.Certificate, err error) {
	sylog.Debugf("cert:[%s] issuer:[%s]", cert.Subject.String(), issuer.Subject.String())

	if !issuer.IsCA {
		return nil, fmt.Errorf("signer's certificates can only belong to a CA")
	}

	// Extract OCSP Server from the certificate in question
	if len(cert.OCSPServer) == 0 {
		return nil, fmt.Errorf("certificate does not support OCSP")
	}

	// RFC 5280, 4.2.2.1 (Authority Information Access)
	ocspURL, err := url.Parse(cert.OCSPServer[0])
	if err != nil {
		return nil, fmt.Errorf("cannot parse OCSP Server from certificate. err: %w", err)
	}

	//  Create OCSP Request

	// Hash contains the hash function that should be used when
	// constructing the OCSP request. If zero, SHA-1 will be used.
	opts := &ocsp.RequestOptions{Hash: crypto.SHA1}

	buffer, err := ocsp.CreateRequest(cert, issuer, opts)
	if err != nil {
		return nil, fmt.Errorf("OCSP Create Request err: %w", err)
	}

	httpRequest, err := http.NewRequest(http.MethodPost, cert.OCSPServer[0], bytes.NewBuffer(buffer))
	if err != nil {
		return nil, fmt.Errorf("HTTP Create Request err: %w", err)
	}

	// Submit OCSP Request
	httpRequest.Header.Add("Content-Type", "application/ocsp-request")
	httpRequest.Header.Add("Accept", "application/ocsp-response")
	httpRequest.Header.Add("host", ocspURL.Host)

	httpClient := &http.Client{}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("OCSP Send Request err: %w", err)
	}

	defer httpResponse.Body.Close()

	// Parse OCSP Response
	output, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body. err: %w", err)
	}

	ocspResponse, err := ocsp.ParseResponseForCert(output, cert, issuer)
	if err != nil {
		return nil, fmt.Errorf("OCSP response err: %w", err)
	}

	// Handle OCSP Response

	// 4.2.2.1  Time
	//
	// - Responses whose thisUpdate time is later than the local system time
	// SHOULD be considered unreliable.
	// - Responses whose nextUpdate value is earlier than
	// the local system time value SHOULD be considered unreliable.
	// - Responses where the nextUpdate value is not set are equivalent to a CRL
	// with no time for nextUpdate (see Section 2.4).
	if ocspResponse.ThisUpdate.After(time.Now()) {
		return nil, fmt.Errorf("unreliable OCSP response")
	}

	if !ocspResponse.NextUpdate.IsZero() {
		if ocspResponse.NextUpdate.Before(time.Now()) {
			return nil, fmt.Errorf("unreliable OCSP response")
		}
	}
	//   If nextUpdate is not set, the responder is indicating that newer
	//   revocation information is available all the time.

	// The OCSP's certificate is signed by a third-party issuer that we need to verify.
	if ocspResponse.Certificate != nil {
		needsValidation = ocspResponse.Certificate
	}

	// Check validity
	switch ocspResponse.Status {
	case ocsp.Good: // means the certificate is still valid
		return needsValidation, nil

	case ocsp.Revoked: // says the certificate was revoked and cannot be trusted
		return needsValidation, fmt.Errorf("certificate revoked at '%s'. Revocation reason code: '%d'",
			ocspResponse.RevokedAt, ocspResponse.RevocationReason)

	default: // states that the server does not know about the requested certificate,
		return needsValidation, fmt.Errorf("status unknown. certificate cannot be trusted")
	}
}
