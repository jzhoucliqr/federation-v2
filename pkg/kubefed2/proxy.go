/*
Copyright 2014 The Kubernetes Authors.

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

package kubefed2

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/proxy"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	defaultPort = 8001
	proxyLong   = `
		Creates a proxy server or application-level gateway between localhost and
		the Kubernetes API Server. It also allows serving static content over specified
		HTTP path. All incoming data enters through one port and gets forwarded to
		the remote kubernetes API Server port, except for the path matching the static content path.`

	proxyExample = `
		# To proxy all of the kubernetes api and nothing else, use:

		    $ kubectl proxy --api-prefix=/

		# To proxy only part of the kubernetes api and also some static files:

		    $ kubectl proxy --www=/my/files --www-prefix=/static/ --api-prefix=/api/

		# The above lets you 'curl localhost:8001/api/v1/pods'.

		# To proxy the entire kubernetes api at a different root, use:

		    $ kubectl proxy --api-prefix=/custom/

		# The above lets you 'curl localhost:8001/custom/api/v1/pods'

		# Run a proxy to kubernetes apiserver on port 8011, serving static content from ./local/www/
		kubectl proxy --port=8011 --www=./local/www/

		# Run a proxy to kubernetes apiserver on an arbitrary local port.
		# The chosen port for the server will be output to stdout.
		kubectl proxy --port=0

		# Run a proxy to kubernetes apiserver, changing the api prefix to k8s-api
		# This makes e.g. the pods api available at localhost:8001/k8s-api/v1/pods/
		kubectl proxy --api-prefix=/k8s-api`
)

func NewCmdProxy(cmdOut io.Writer, config util.FedConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use: "proxy [--port=PORT] [--www=static-dir] [--www-prefix=prefix] [--api-prefix=prefix]",
		DisableFlagsInUseLine: true,
		Short:   "Run a proxy to the Kubernetes API server",
		Long:    proxyLong,
		Example: proxyExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunProxy(cmdOut, cmd)
			if err != nil {
				glog.Fatalf("error: %v", err)
			}
		},
	}
	cmd.Flags().StringP("www", "w", "", "Also serve static files from the given directory under the specified prefix.")
	cmd.Flags().StringP("www-prefix", "P", "/static/", "Prefix to serve static files under, if static file directory is specified.")
	cmd.Flags().StringP("api-prefix", "", "/", "Prefix to serve the proxied API under.")
	cmd.Flags().String("accept-paths", proxy.DefaultPathAcceptRE, "Regular expression for paths that the proxy should accept.")
	cmd.Flags().String("reject-paths", proxy.DefaultPathRejectRE, "Regular expression for paths that the proxy should reject. Paths specified here will be rejected even accepted by --accept-paths.")
	cmd.Flags().String("accept-hosts", proxy.DefaultHostAcceptRE, "Regular expression for hosts that the proxy should accept.")
	cmd.Flags().String("reject-methods", proxy.DefaultMethodRejectRE, "Regular expression for HTTP methods that the proxy should reject (example --reject-methods='POST,PUT,PATCH'). ")
	cmd.Flags().IntP("port", "p", defaultPort, "The port on which to run the proxy. Set to 0 to pick a random port.")
	cmd.Flags().StringP("address", "", "127.0.0.1", "The IP address on which to serve on.")
	cmd.Flags().Bool("disable-filter", false, "If true, disable request filtering in the proxy. This is dangerous, and can leave you vulnerable to XSRF attacks, when used with an accessible port.")
	cmd.Flags().StringP("unix-socket", "u", "", "Unix socket on which to run the proxy.")
	cmd.Flags().Duration("keepalive", 0, "keepalive specifies the keep-alive period for an active network connection. Set to 0 to disable keepalive.")
	return cmd
}

func RunProxy(out io.Writer, cmd *cobra.Command) error {
	path := util.GetFlagString(cmd, "unix-socket")
	port := util.GetFlagInt(cmd, "port")
	address := util.GetFlagString(cmd, "address")

	if port != defaultPort && path != "" {
		return errors.New("Don't specify both --unix-socket and --port")
	}

	clientConfig, err := getDefaultRESTConfig()
	if err != nil {
		return err
	}

	staticPrefix := util.GetFlagString(cmd, "www-prefix")
	if !strings.HasSuffix(staticPrefix, "/") {
		staticPrefix += "/"
	}
	staticDir := util.GetFlagString(cmd, "www")
	if staticDir != "" {
		fileInfo, err := os.Stat(staticDir)
		if err != nil {
			glog.Warning("Failed to stat static file directory "+staticDir+": ", err)
		} else if !fileInfo.IsDir() {
			glog.Warning("Static file directory " + staticDir + " is not a directory")
		}
	}

	apiProxyPrefix := util.GetFlagString(cmd, "api-prefix")
	if !strings.HasSuffix(apiProxyPrefix, "/") {
		apiProxyPrefix += "/"
	}
	filter := &proxy.FilterServer{
		AcceptPaths:   proxy.MakeRegexpArrayOrDie(util.GetFlagString(cmd, "accept-paths")),
		RejectPaths:   proxy.MakeRegexpArrayOrDie(util.GetFlagString(cmd, "reject-paths")),
		AcceptHosts:   proxy.MakeRegexpArrayOrDie(util.GetFlagString(cmd, "accept-hosts")),
		RejectMethods: proxy.MakeRegexpArrayOrDie(util.GetFlagString(cmd, "reject-methods")),
	}
	if util.GetFlagBool(cmd, "disable-filter") {
		if path == "" {
			glog.Warning("Request filter disabled, your proxy is vulnerable to XSRF attacks, please be cautious")
		}
		filter = nil
	}

	keepalive := util.GetFlagDuration(cmd, "keepalive")

	server, err := proxy.NewServer(staticDir, apiProxyPrefix, staticPrefix, filter, clientConfig, keepalive)

	// Separate listening from serving so we can report the bound port
	// when it is chosen by os (eg: port == 0)
	var l net.Listener
	if path == "" {
		l, err = server.Listen(address, port)
	} else {
		l, err = server.ListenUnix(path)
	}
	if err != nil {
		glog.Fatal(err)
	}
	fmt.Fprintf(out, "Starting to serve on %s\n", l.Addr().String())
	glog.Fatal(server.ServeOnListener(l))
	return nil
}

func getDefaultRESTConfig() (*rest.Config, error) {
	// try incluster first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// try load default config from file
	cfgFile := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	return clientcmd.BuildConfigFromFlags("", cfgFile)
}
