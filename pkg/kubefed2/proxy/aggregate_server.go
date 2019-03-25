package proxy

import (
	"encoding/json"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1b1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	jsonse "k8s.io/apimachinery/pkg/runtime/serializer/json"
	//"k8s.io/client-go/kubernetes/scheme"
)

type aggregateServer struct {
	filebase            string
	apiProxyPrefix      string
	staticPrefix        string
	filter              *FilterServer
	clusterNames        []string
	cfgs                []*rest.Config
	impersonationConfig *transport.ImpersonationConfig
	namespace           string
	resourceType        string
	resourceName        string
	keepalive           time.Duration
}

func newAggregateProxyHandler(filebase string, apiProxyPrefix string, staticPrefix string, filter *FilterServer, clusterNames []string, cfgs []*rest.Config, keepalive time.Duration, impersonationConfig *transport.ImpersonationConfig, namespace, resourceType, resourceName string) (http.Handler, error) {
	return &aggregateServer{
		filebase:            filebase,
		apiProxyPrefix:      apiProxyPrefix,
		staticPrefix:        staticPrefix,
		filter:              filter,
		clusterNames:        clusterNames,
		cfgs:                cfgs,
		impersonationConfig: impersonationConfig,
		namespace:           namespace,
		resourceType:        resourceType,
		resourceName:        resourceName,
	}, nil
}

func (S *aggregateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	responseWriters := []*httptest.ResponseRecorder{}

	for _, cfg := range S.cfgs {
		wp := httptest.NewRecorder()
		responseWriters = append(responseWriters, wp)
		proxyToTarget, err := newDynamicProxyHandler(S.filebase, S.apiProxyPrefix, S.staticPrefix, S.filter, cfg, S.keepalive, S.impersonationConfig)
		if err != nil {
			w.WriteHeader(501)
			return
		}
		proxyToTarget.ServeHTTP(wp, r)
	}

	metaFactory := jsonse.DefaultMetaFactory

	//var table := &metav1b1.Table{}
	var table *metav1b1.Table
	var list *metav1.List

	for i, wp := range responseWriters {
		clusterName := S.clusterNames[i]
		resp := wp.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		glog.V(1).Infof("status: %d", resp.StatusCode)
		glog.V(1).Infof(resp.Header.Get("Content-Type"))
		//glog.V(1).Infof(string(body))

		gvk, err := metaFactory.Interpret(body)
		if err != nil {
			glog.Errorf("%v", err)
		}

		glog.V(1).Infof("gvk: %v", gvk)
		if gvk.Kind == "Table" {
			if i == 0 {
				table = &metav1b1.Table{}
				if err := json.Unmarshal(body, table); err != nil {
					glog.Errorf("%v", err)
				}
				addClusterToTable(table)
				for j := range table.Rows {
					table.Rows[j].Cells = append(table.Rows[j].Cells, clusterName)
				}
				saveTableRowsRoutes(table.Rows, clusterName)
			} else {
				ti := &metav1b1.Table{}
				if err := json.Unmarshal(body, ti); err != nil {
					glog.Errorf("%v", err)
				}
				for j := range ti.Rows {
					ti.Rows[j].Cells = append(ti.Rows[j].Cells, clusterName)
				}
				table.Rows = append(table.Rows, ti.Rows...)
				saveTableRowsRoutes(ti.Rows, clusterName)
			}
		} else if strings.HasSuffix(gvk.Kind, "List") {
			if i == 0 {
				list = &metav1.List{}
				if err := json.Unmarshal(body, list); err != nil {
					glog.Errorf("%v", err)
				}
				saveListItemsRoutes(list.Items, clusterName)
			} else {
				ti := &metav1.List{}
				if err := json.Unmarshal(body, ti); err != nil {
					glog.Errorf("%v", err)
				}
				list.Items = append(list.Items, ti.Items...)
				saveListItemsRoutes(ti.Items, clusterName)
			}
		} else {
			glog.Infof("not table, not list, oops!")
			return
		}
	}

	if list != nil {
		dataAll, err := json.Marshal(list)
		if err != nil {
			glog.Errorf("%v", err)
		}
		glog.V(1).Infof("data: %s", string(dataAll))
		w.Header().Add("Content-Type", "application/json")
		w.Write(dataAll)
	} else {
		dataAll, err := json.Marshal(table)
		if err != nil {
			glog.Errorf("%v", err)
		}
		glog.V(1).Infof("data: %s", string(dataAll))
		w.Header().Add("Content-Type", "application/json")
		w.Write(dataAll)
	}
}

func saveTableRowsRoutes(rows []metav1b1.TableRow, clusterName string) {
}

func saveListItemsRoutes(items []runtime.RawExtension, clusterName string) {
}

func addClusterToTable(table *metav1b1.Table) {
	column := metav1b1.TableColumnDefinition{
		Name:        "Cluster",
		Type:        "string",
		Description: "Cluster name which this resource belongs to",
	}
	table.ColumnDefinitions = append(table.ColumnDefinitions, column)
}
