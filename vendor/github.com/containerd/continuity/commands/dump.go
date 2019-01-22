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
	"os"

	pb "github.com/containerd/continuity/proto"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
)

var DumpCmd = &cobra.Command{
	Use:   "dump <manifest>",
	Short: "Dump the contents of the manifest in protobuf text format",
	Run: func(cmd *cobra.Command, args []string) {
		var p []byte
		var err error

		if len(args) < 1 {
			p, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("error reading manifest: %v", err)
			}
		} else {
			p, err = ioutil.ReadFile(args[0])
			if err != nil {
				log.Fatalf("error reading manifest: %v", err)
			}
		}

		var bm pb.Manifest

		if err := proto.Unmarshal(p, &bm); err != nil {
			log.Fatalf("error unmarshaling manifest: %v", err)
		}

		// TODO(stevvooe): For now, just dump the text format. Turn this into
		// nice text output later.
		if err := proto.MarshalText(os.Stdout, &bm); err != nil {
			log.Fatalf("error dumping manifest: %v", err)
		}
	},
}
