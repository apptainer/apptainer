// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

const (
	userServicePath = "/v1/rbac/users/current"
)

type userOidcMeta struct {
	Secret string `json:"secret"`
}

type userData struct {
	OidcMeta userOidcMeta `json:"oidc_user_meta"`
	Email    string       `json:"email"`
	Realname string       `json:"realname"`
	Username string       `json:"username"`
}
