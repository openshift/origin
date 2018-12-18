/*
   Copyright The containerd Authors.

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

package commands

import (
	"io/ioutil"
	"log"

	"github.com/containerd/continuity"
	"github.com/spf13/cobra"
)

var VerifyCmd = &cobra.Command{
	Use:   "verify <root> [<manifest>]",
	Short: "Verify the root against the provided manifest",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			log.Fatalln("please specify a root and manifest")
		}

		root, path := args[0], args[1]

		p, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalf("error reading manifest: %v", err)
		}

		m, err := continuity.Unmarshal(p)
		if err != nil {
			log.Fatalf("error unmarshaling manifest: %v", err)
		}

		ctx, err := continuity.NewContext(root)
		if err != nil {
			log.Fatalf("error getting context: %v", err)
		}

		if err := continuity.VerifyManifest(ctx, m); err != nil {
			// TODO(stevvooe): Support more interesting error reporting.
			log.Fatalf("error verifying manifest: %v", err)
		}
	},
}
