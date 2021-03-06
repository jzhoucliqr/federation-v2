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

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"
	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/core/v1alpha1"
	proxyv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/proxy/v1alpha1"
	genericclient "github.com/kubernetes-sigs/federation-v2/pkg/client/generic"
	ctlutil "github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
	certutil "k8s.io/client-go/util/cert"
)

const (
	// DefaultHostAcceptRE is the default value for which hosts to accept.
	DefaultHostAcceptRE = "^localhost$,^127\\.0\\.0\\.1$,^\\[::1\\]$"
	// DefaultPathAcceptRE is the default path to accept.
	DefaultPathAcceptRE = "^.*"
	// DefaultPathRejectRE is the default set of paths to reject.
	//DefaultPathRejectRE = "^/api/.*/pods/.*/exec,^/api/.*/pods/.*/attach"
	DefaultPathRejectRE = "^/api/.*/pods/.*/attach"
	// DefaultMethodRejectRE is the set of HTTP methods to reject by default.
	DefaultMethodRejectRE = "^$"
)

var (
	// ReverseProxyFlushInterval is the frequency to flush the reverse proxy.
	// Only matters for long poll connections like the one used to watch. With an
	// interval of 0 the reverse proxy will buffer content sent on any connection
	// with transfer-encoding=chunked.
	// TODO: Flush after each chunk so the client doesn't suffer a 100ms latency per
	// watch event.
	ReverseProxyFlushInterval = 100 * time.Millisecond
)

// FilterServer rejects requests which don't match one of the specified regular expressions
type FilterServer struct {
	// Only paths that match this regexp will be accepted
	AcceptPaths []*regexp.Regexp
	// Paths that match this regexp will be rejected, even if they match the above
	RejectPaths []*regexp.Regexp
	// Hosts are required to match this list of regexp
	AcceptHosts []*regexp.Regexp
	// Methods that match this regexp are rejected
	RejectMethods []*regexp.Regexp
	// The delegate to call to handle accepted requests.
	delegate http.Handler
}

// MakeRegexpArray splits a comma separated list of regexps into an array of Regexp objects.
func MakeRegexpArray(str string) ([]*regexp.Regexp, error) {
	parts := strings.Split(str, ",")
	result := make([]*regexp.Regexp, len(parts))
	for ix := range parts {
		re, err := regexp.Compile(parts[ix])
		if err != nil {
			return nil, err
		}
		result[ix] = re
	}
	return result, nil
}

// MakeRegexpArrayOrDie creates an array of regular expression objects from a string or exits.
func MakeRegexpArrayOrDie(str string) []*regexp.Regexp {
	result, err := MakeRegexpArray(str)
	if err != nil {
		glog.Fatalf("Error compiling re: %v", err)
	}
	return result
}

func matchesRegexp(str string, regexps []*regexp.Regexp) bool {
	for _, re := range regexps {
		if re.MatchString(str) {
			glog.V(6).Infof("%v matched %s", str, re)
			return true
		}
	}
	return false
}

func (f *FilterServer) accept(method, path, host string) bool {
	if matchesRegexp(path, f.RejectPaths) {
		glog.V(3).Infof("Filter rejecting for path %v %v", path, f.RejectPaths)
		return false
	}
	if matchesRegexp(method, f.RejectMethods) {
		glog.V(3).Infof("Filter rejecting for methods %v %v", method, f.RejectMethods)
		return false
	}
	if matchesRegexp(path, f.AcceptPaths) && matchesRegexp(host, f.AcceptHosts) {
		return true
	}
	return false
}

// HandlerFor makes a shallow copy of f which passes its requests along to the
// new delegate.
func (f *FilterServer) HandlerFor(delegate http.Handler) *FilterServer {
	f2 := *f
	f2.delegate = delegate
	return &f2
}

// Get host from a host header value like "localhost" or "localhost:8080"
func extractHost(header string) (host string) {
	host, _, err := net.SplitHostPort(header)
	if err != nil {
		host = header
	}
	return host
}

func (f *FilterServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	host := extractHost(req.Host)
	if f.accept(req.Method, req.URL.Path, host) {
		glog.V(3).Infof("Filter accepting %v %v %v", req.Method, req.URL.Path, host)
		f.delegate.ServeHTTP(rw, req)
		return
	}
	glog.V(3).Infof("Filter rejecting %v %v %v", req.Method, req.URL.Path, host)
	http.Error(rw, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}

// Server is a http.Handler which proxies Kubernetes APIs to remote API server.
type Server struct {
	handler http.Handler

	// client to central registry
	// used to get neighbor clusters
	// also used to get namespace placement
	client *genericclient.Client
}

type responder struct{}

func (r *responder) Error(w http.ResponseWriter, req *http.Request, err error) {
	glog.Errorf("Error while proxying request: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// makeUpgradeTransport creates a transport that explicitly bypasses HTTP2 support
// for proxy connections that must upgrade.
func makeUpgradeTransport(config *rest.Config, keepalive time.Duration) (proxy.UpgradeRequestRoundTripper, error) {
	transportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(transportConfig)
	if err != nil {
		return nil, err
	}
	rt := utilnet.SetOldTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: keepalive,
		}).DialContext,
	})

	upgrader, err := transport.HTTPWrappersForConfig(transportConfig, proxy.MirrorRequest)
	if err != nil {
		return nil, err
	}

	impersonateConfig := transport.ImpersonationConfig{
		UserName: "minikube-user",
	}
	impersonateTransport := transport.NewImpersonatingRoundTripper(impersonateConfig, rt)

	//return proxy.NewUpgradeRequestRoundTripper(rt, impersonateTransport), nil
	return proxy.NewUpgradeRequestRoundTripper(impersonateTransport, upgrader), nil
}

// NewServer creates and installs a new Server.
// 'filter', if non-nil, protects requests to the api only.
func NewServer(filebase string, apiProxyPrefix string, staticPrefix string, filter *FilterServer, cfg *rest.Config, keepalive time.Duration) (*Server, error) {
	// init generic client, get secret using cfg, then get config to central cluster
	config, err := getCentralRegistryConfig(cfg)
	if err != nil {
		return nil, err
	}

	glog.V(1).Info("after get central config")
	client := genericclient.NewForConfigOrDieWithUserAgent(config, "Proxy")
	glog.V(1).Info("after get client ")

	secret := &apiv1.Secret{}
	err = client.Get(context.TODO(), secret, "default", "kubeconfig-central")
	if err != nil {
		return nil, err
	}

	proxyServer, err := newTopProxyHandler(&client, filebase, apiProxyPrefix, staticPrefix, filter, cfg, keepalive)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(apiProxyPrefix, proxyServer)
	if filebase != "" {
		// Require user to explicitly request this behavior rather than
		// serving their working directory by default.
		mux.Handle(staticPrefix, newFileHandler(staticPrefix, filebase))
	}

	return &Server{
		handler: mux,
		client:  &client,
	}, nil
}

func getCentralRegistryConfig(cfg *rest.Config) (*rest.Config, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	namespace := "federation-system"
	secret, err := clientset.CoreV1().Secrets(namespace).Get("kubeconfig-central", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	kubeconfigGetter := ctlutil.KubeconfigGetterForSecret(secret)
	centralConfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfigGetter)
	return centralConfig, err
}

func newAuthenticatorFromClientCAFile(clientCAFile string) (authenticator.Request, error) {
	roots, err := certutil.NewPool(clientCAFile)
	if err != nil {
		return nil, err
	}

	opts := x509.DefaultVerifyOptions()
	opts.Roots = roots

	return x509.New(opts, x509.CommonNameUserConversion), nil
}

func newTopProxyHandler(centralclient *genericclient.Client, filebase string, apiProxyPrefix string, staticPrefix string, filter *FilterServer, cfg *rest.Config, keepalive time.Duration) (http.Handler, error) {
	proxyServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glog.V(1).Infof("in top handler, use new cfg")
		clientCAFile := "/var/lib/minikube/certs/ca.crt"
		authenticator, err := newAuthenticatorFromClientCAFile(clientCAFile)
		if err != nil {
			w.WriteHeader(501)
			return
		}
		authResp, ok, err := authenticator.AuthenticateRequest(r)
		glog.V(1).Infof("%v, %v, %v", authResp, ok, err)

		if err != nil {
			w.WriteHeader(501)
			return
		}
		if !ok {
			w.WriteHeader(401)
			return
		}

		impersonateConfig := &transport.ImpersonationConfig{
			UserName: authResp.User.GetName(),
			Groups:   authResp.User.GetGroups(),
			Extra:    authResp.User.GetExtra(),
		}

		namespace, resourceType, resourceName := getNamespaceTypeNameFromPath(r.URL.Path)
		glog.V(1).Infof("namespace: %s, type: %s, name: %s", namespace, resourceType, resourceName)
		var clusterName string
		var proxyCfg *rest.Config
		if namespace == "" {
			glog.V(1).Infof("no namespace, proxy to local")
			proxyCfg = cfg
		} else {
			if needToGetFromMaster(resourceType) {
				glog.V(1).Infof("get from master for resource type: %s", resourceType)
				clusterName, err = getClusterFromNamespace(*centralclient, namespace)
				if err != nil {
					glog.Errorf("%v", err)
				}
				glog.V(1).Infof("cluster for namespace [%s] is [%s]", namespace, clusterName)
				proxyCfg, err = getRestConfigForCluster(*centralclient, clusterName)
				if err != nil {
					glog.Errorf("%v", err)
				}
			} else if resourceName != "" && getClusterFromCache(namespace, resourceType, resourceName) != "" {
				clusterName = getClusterFromCache(namespace, resourceType, resourceName)
				// get from one worker
				glog.V(1).Infof("get from a single worker for resource type: %s", resourceType)
				glog.V(1).Infof("cluster for namespace [%s] is [%s]", namespace, clusterName)
				proxyCfg, err = getRestConfigForCluster(*centralclient, clusterName)
				if err != nil {
					glog.Errorf("%v", err)
				}
			} else {
				// get from all workers
				glog.V(1).Infof("get from all workers for resource type: %s", resourceType)
				clusterNames, err := getWorkerClustersFromNamespace(*centralclient, namespace)
				if err != nil {
					glog.Errorf("%v", err)
				}
				glog.V(1).Infof("cluster for namespace [%s] is [%v]", namespace, clusterNames)
				if len(clusterNames) == 1 {
					proxyCfg, err = getRestConfigForCluster(*centralclient, clusterNames[0])
					if err != nil {
						glog.Errorf("%v", err)
					}
				} else {
					restConfigs, err := getRestConfigsForClusters(*centralclient, clusterNames)
					if err != nil {
						glog.Errorf("%v", err)
					}
					proxyAggregate, err := newAggregateProxyHandler(filebase, apiProxyPrefix, staticPrefix, filter, clusterNames, restConfigs, keepalive, impersonateConfig, namespace, resourceType, resourceName)
					if err != nil {
						glog.Errorf("%v", err)
					}

					proxyAggregate.ServeHTTP(w, r)
				}
			}
		}

		if proxyCfg != nil {
			// get from single cluster, either local, or master, or one of the worker
			proxyToTarget, err := newDynamicProxyHandler(filebase, apiProxyPrefix, staticPrefix, filter, proxyCfg, keepalive, impersonateConfig)
			if err != nil {
				w.WriteHeader(501)
				return
			}
			proxyToTarget.ServeHTTP(w, r)
		}
	})

	return proxyServer, nil
}

func newDynamicProxyHandler(filebase string, apiProxyPrefix string, staticPrefix string, filter *FilterServer, cfg *rest.Config, keepalive time.Duration, impersonateConfig *transport.ImpersonationConfig) (http.Handler, error) {

	glog.V(1).Infof("in dynamic handler")
	host := cfg.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	responder := &responder{}
	resttransport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, err
	}

	impersonateTransport := transport.NewImpersonatingRoundTripper(*impersonateConfig, resttransport)

	upgradeTransport, err := makeUpgradeTransport(cfg, keepalive)
	if err != nil {
		return nil, err
	}
	proxy := proxy.NewUpgradeAwareHandler(target, impersonateTransport, false, false, responder)
	//proxy := proxy.NewUpgradeAwareHandler(target, resttransport, false, false, responder)
	proxy.UpgradeTransport = upgradeTransport
	proxy.UseRequestLocation = true

	proxyServer := http.Handler(proxy)
	if filter != nil {
		proxyServer = filter.HandlerFor(proxyServer)
	}

	if !strings.HasPrefix(apiProxyPrefix, "/api") {
		proxyServer = stripLeaveSlash(apiProxyPrefix, proxyServer)
	}

	return proxyServer, nil
}

// Listen is a simple wrapper around net.Listen.
func (s *Server) Listen(address string, port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
}

// ListenUnix does net.Listen for a unix socket
func (s *Server) ListenUnix(path string) (net.Listener, error) {
	// Remove any socket, stale or not, but fall through for other files
	fi, err := os.Stat(path)
	if err == nil && (fi.Mode()&os.ModeSocket) != 0 {
		os.Remove(path)
	}
	// Default to only user accessible socket, caller can open up later if desired
	oldmask, _ := umask(0077)
	l, err := net.Listen("unix", path)
	umask(oldmask)
	return l, err
}

// ServeOnListener starts the server using given listener, loops forever.
func (s *Server) ServeOnListener(l net.Listener) error {
	clientca := "/var/lib/minikube/certs/ca.crt"
	cert := "/var/lib/minikube/certs/apiserver.crt"
	key := "/var/lib/minikube/certs/apiserver.key"
	roots, err := certutil.NewPool(clientca)
	if err != nil {
		return err
	}

	server := http.Server{
		Handler: s.handler,
		TLSConfig: &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  roots,
		},
	}
	return server.ServeTLS(l, cert, key)
}

func newFileHandler(prefix, base string) http.Handler {
	return http.StripPrefix(prefix, http.FileServer(http.Dir(base)))
}

// like http.StripPrefix, but always leaves an initial slash. (so that our
// regexps will work.)
func stripLeaveSlash(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, prefix)
		if len(p) >= len(req.URL.Path) {
			http.NotFound(w, req)
			return
		}
		if len(p) > 0 && p[:1] != "/" {
			p = "/" + p
		}
		req.URL.Path = p
		h.ServeHTTP(w, req)
	})
}

func getNamespaceTypeNameFromPath(path string) (string, string, string) {
	glog.V(1).Infof("path: %s", path)
	r, _ := regexp.Compile(".*/namespaces/([^/]*)/([^/]*)/?(.*)")
	sub := r.FindStringSubmatch(path)
	if len(sub) != 4 {
		return "", "", ""
	}
	return sub[1], sub[2], sub[3]
}

func getClusterFromNamespace(centralclient genericclient.Client, namespace string) (string, error) {
	placement := &proxyv1a1.NamespacePlacement{}
	err := centralclient.Get(context.TODO(), placement, "federation-system", namespace)
	if err != nil {
		return "", err
	}

	return placement.Spec.MasterCluster, nil
}

func getRestConfigForCluster(client genericclient.Client, clusterName string) (*rest.Config, error) {
	//newCfg, err := clientcmd.BuildConfigFromFlags("", newCfgFile)
	fedNamespace := "federation-system"
	clusterNamespace := "kube-multicluster-public"
	fedCluster := &fedv1a1.FederatedCluster{}
	err := client.Get(context.TODO(), fedCluster, fedNamespace, clusterName)
	if err != nil {
		return nil, err
	}

	config, err := ctlutil.BuildClusterConfig(fedCluster, client, fedNamespace, clusterNamespace)
	return config, err
}

// Umask is a wrapper for `unix.Umask()` on non-Windows platforms
func umask(mask int) (old int, err error) {
	return unix.Umask(mask), nil
}

func needToGetFromMaster(resourceType string) bool {
	return strings.HasPrefix(resourceType, "Federated")
}

//cache for routes to clusters
var clusterCache map[string]string = map[string]string{}

func getClusterFromCache(namespace, resourceType, resourceName string) string {
	key := fmt.Sprintf("%s/%s/%s", namespace, resourceType, resourceName)
	return clusterCache[key]
}

func getWorkerClustersFromNamespace(centralclient genericclient.Client, namespace string) ([]string, error) {
	placement := &proxyv1a1.NamespacePlacement{}
	err := centralclient.Get(context.TODO(), placement, "federation-system", namespace)
	if err != nil {
		return nil, err
	}

	return placement.Spec.WorkerClusters, nil
}

func getRestConfigsForClusters(client genericclient.Client, clusterNames []string) ([]*rest.Config, error) {
	//newCfg, err := clientcmd.BuildConfigFromFlags("", newCfgFile)
	configs := []*rest.Config{}
	fedNamespace := "federation-system"
	clusterNamespace := "kube-multicluster-public"
	for _, clusterName := range clusterNames {
		fedCluster := &fedv1a1.FederatedCluster{}
		err := client.Get(context.TODO(), fedCluster, fedNamespace, clusterName)
		if err != nil {
			glog.Errorf("%v", err)
			continue
		}

		config, err := ctlutil.BuildClusterConfig(fedCluster, client, fedNamespace, clusterNamespace)
		if err != nil {
			glog.Errorf("%v", err)
			continue
		}
		if config != nil {
			configs = append(configs, config)
		}
	}
	return configs, nil
}
