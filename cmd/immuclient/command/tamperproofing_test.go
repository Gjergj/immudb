/*
Copyright 2019-2020 vChain, Inc.

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

package immuclient

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	test "github.com/codenotary/immudb/cmd/immuclient/immuclienttest"
	"github.com/codenotary/immudb/pkg/server"
	"github.com/codenotary/immudb/pkg/server/servertest"
	"github.com/spf13/cobra"
)

func TestConsistency(t *testing.T) {
	options := server.Options{}.WithAuth(true).WithInMemoryStore(true)
	bs := servertest.NewBufconnServer(options)
	bs.Start()

	ic := test.NewClientTest(&test.PasswordReader{
		Pass: []string{"immudb"},
	}, &test.HomedirServiceMock{})
	ic.Connect(bs.Dialer)
	ic.Login("immudb")

	cmdl := commandline{
		immucl: ic.Imc,
	}
	cmd := cobra.Command{}
	cmdl.consistency(&cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	setmsg, err := cmdl.immucl.SafeSet([]string{"key", "value"})
	hash := strings.Split(setmsg, "hash:		")[1]
	hash = hash[:64]

	cmd.SetArgs([]string{"check-consistency", "0", hash})
	err = cmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
	msg, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "firstRoot") {
		t.Fatal(err)
	}
}
func TestInclusion(t *testing.T) {
	options := server.Options{}.WithAuth(true).WithInMemoryStore(true)
	bs := servertest.NewBufconnServer(options)
	bs.Start()

	ic := test.NewClientTest(&test.PasswordReader{
		Pass: []string{"immudb"},
	}, &test.HomedirServiceMock{})
	ic.Connect(bs.Dialer)
	ic.Login("immudb")

	cmdl := commandline{
		immucl: ic.Imc,
	}
	cmd := cobra.Command{}
	cmdl.inclusion(&cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	setmsg, err := cmdl.immucl.SafeSet([]string{"key", "value"})
	hash := strings.Split(setmsg, "hash:		")[1]
	hash = hash[:64]

	cmd.SetArgs([]string{"inclusion", "0"})
	err = cmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
	msg, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "verified: true") {
		t.Fatal(err)
	}
}
