// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package testdata holds files, data and code for testing.
// KEYS IN THIS FILE ARE ONLY FOR TESTING. They must not be used by production code.
package testdata

// DemoPrivateKeyPass is the password for DemoPrivateKey
const DemoPrivateKeyPass string = "towel"

// DemoPrivateKey is the private key itself; must only be used for testing purposes
const DemoPrivateKey string = `
-----BEGIN EC PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: DES-CBC,B71ECAB011EB4E8F

+6cz455aVRHFX5UsxplyGvFXMcmuMH0My/nOWNmYCL+bX2PnHdsv3dRgpgPRHTWt
IPI6kVHv0g2uV5zW8nRqacmikBFA40CIKp0SjRmi1CtfchzuqXQ3q40rFwCjeuiz
t48+aoeFsfU6NnL5sP8mbFlPze+o7lovgAWEqHEcebU=
-----END EC PRIVATE KEY-----`

// DemoPublicKey is the public key that corresponds to DemoPrivateKey.
const DemoPublicKey string = `
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEsAVg3YB0tOFf3DdC2YHPL2WiuCNR
1iywqGjjtu2dAdWktWqgRO4NTqPJXUggSQL3nvOupHB4WZFZ4j3QhtmWRg==
-----END PUBLIC KEY-----`
