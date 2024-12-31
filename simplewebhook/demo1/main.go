package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	// 项目根目录下创建webhooks, 用于存存放不同resource的valiate，mutate逻辑
	"my-webhook/webhooks"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type serverParams struct {
	port     int
	certFile string
	keyFile  string
}

type WebhookServer struct {
	server *http.Server
}

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func (ws *WebhookServer) validate(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}
	ar := &v1beta1.AdmissionReview{}
	var admissionResponse *v1beta1.AdmissionResponse
	if _, _, err := deserializer.Decode(body, nil, ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = webhooks.Validate(ar) // validation
	}
	ar.Response = admissionResponse
	resp, _ := json.Marshal(ar)
	w.Write(resp)
}

func (ws *WebhookServer) mutate(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}
	ar := &v1beta1.AdmissionReview{}
	var admissionResponse *v1beta1.AdmissionResponse
	if _, _, err := deserializer.Decode(body, nil, ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = webhooks.Mutate(ar) // mutate
	}
	ar.Response = admissionResponse
	resp, _ := json.Marshal(ar)
	w.Write(resp)
}

func main() {
	var parameters serverParams
	// 指定webhook server的端口，和使用的证书的位置
	flag.IntVar(&parameters.port, "port", 9443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		glog.Errorf("Failed to load key pair: %v", err)
	}

	whsvr := &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", parameters.port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", whsvr.validate)
	mux.HandleFunc("/mutate", whsvr.mutate)
	mux.HandleFunc("/healthcheck", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("running"))
	}))
	whsvr.server.Handler = mux

	// start webhook server in new routine
	if err := whsvr.server.ListenAndServeTLS("", ""); err != nil {
		glog.Errorf("Failed to listen and serve webhook server: %v", err)
	}
}
