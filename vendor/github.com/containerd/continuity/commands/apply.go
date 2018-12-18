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

var ApplyCmd = &cobra.Command{
	Use:   "apply <root> [<manifest>]",
	Short: "Apply the manifest to the provided root",
	Run: func(cmd *cobra.Command, args []string) {
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

		if err := continuity.ApplyManifest(ctx, m); err != nil {
			log.Fatalf("error applying manifest: %v", err)
		}
	},
}
