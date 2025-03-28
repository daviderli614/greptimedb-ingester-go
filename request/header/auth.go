/*
 * Copyright 2024 Greptime Team
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package header

import (
	gpb "github.com/GreptimeTeam/greptime-proto/go/greptime/v1"

	"github.com/GreptimeTeam/greptimedb-ingester-go/util"
)

type Auth struct {
	username string
	password string
}

func newAuth(username, password string) Auth {
	return Auth{
		username: username,
		password: password,
	}
}

// buildAuthHeader only supports Basic Auth so far
func (a Auth) buildAuthHeader() *gpb.AuthHeader {
	if util.IsEmptyString(a.username) || util.IsEmptyString(a.password) {
		return nil
	}

	return &gpb.AuthHeader{
		AuthScheme: &gpb.AuthHeader_Basic{
			Basic: &gpb.Basic{
				Username: a.username,
				Password: a.password,
			},
		},
	}

}
