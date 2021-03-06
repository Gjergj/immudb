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

package immugw

import (
	"os"
	"testing"

	"github.com/codenotary/immudb/cmd/version"
	"github.com/codenotary/immudb/pkg/gw"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewCmd(t *testing.T) {
	version.App = "immugw"

	viper.Set("mtls", true)
	logfile := "./immugw_cmd_test_logfile.log"
	viper.Set("logfile", logfile)
	defer os.Remove(logfile)

	cmd := NewCmd(new(gw.ImmuGwServerMock))
	require.NoError(t, cmd.Execute())
}
