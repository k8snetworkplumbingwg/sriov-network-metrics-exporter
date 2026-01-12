package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "main test suite")
}

var _ = DescribeTable("test endpointOnly handler", // endpointOnly
	func(endpoint string, expectedResponse int) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, endpoint, http.NoBody)
		handler := endpointOnly(promhttp.Handler(), metricsEndpoint)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(expectedResponse))
	},
	Entry("returns status 'OK' when request endpoint is '/metrics'", "/metrics", http.StatusOK),
	Entry("returns status 'Not Found' when request endpoint is not '/metrics'", "/invalidendpoint", http.StatusNotFound),
)

var _ = DescribeTable("test getOnly handler", // getOnly
	func(method string, expectedResponse int) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, metricsEndpoint, http.NoBody)
		handler := getOnly(promhttp.Handler())

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(expectedResponse))
	},
	Entry("returns status 'OK' when request method is 'GET'", http.MethodGet, http.StatusOK),
	Entry("returns status 'MethodNotAllowed' when request method is not 'GET'", http.MethodPost, http.StatusMethodNotAllowed),
)

var _ = DescribeTable("test noBody handler", // noBody
	func(body io.Reader, expectedResponse int) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, metricsEndpoint, body)
		handler := noBody(promhttp.Handler())

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(expectedResponse))
	},
	Entry("returns status 'OK' when request body is empty", nil, http.StatusOK),
	Entry("returns status 'Bad Request' when request body is not empty", bytes.NewReader([]byte("body")), http.StatusBadRequest),
)

var _ = DescribeTable("test limitRequests handler", // limitRequests
	func(limit int, requests int, expectedResponse int) {
		handler := limitRequests(promhttp.Handler(), rate.Limit(limit), limit)

		code := http.StatusOK
		for i := 0; i < requests; i++ {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, metricsEndpoint, http.NoBody)
			handler.ServeHTTP(recorder, request)

			code = recorder.Code
		}

		Expect(code).To(Equal(expectedResponse))
	},
	Entry("returns status 'OK' when the number of requests does not exceed the request limit", 10, 10, http.StatusOK),
	Entry("returns status 'Too Many Requests' when number of requests exceeds the request limit", 10, 11, http.StatusTooManyRequests),
)
