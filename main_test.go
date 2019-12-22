package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRedeploy_ok(t *testing.T) {
	assert := assert.New(t)
	deployToken := "625181dbfb5c6100cdacd97f3ba32ab4"

	dc := &mockDockerClient{
		GetImageIDOutput: "repository/svc:1.0",
	}
	e := &env{
		cfg: Config{
			Authentication: AuthKey{
				Key:  "alg=scrypt$N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739",
				Salt: "478c1d403dec20707cf487f81c06d646",
			},
			Services: map[string]Target{
				"test-svc": Target{
					ID:        "test-svc",
					Binary:    "/bin/sh",
					Script:    "./resources/test-svc.sh",
					MustMatch: "^repository/svc:.*",
				},
			},
		},
		docker: dc,
	}
	server := newServer(e, 9000)

	req1 := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "repository/svc:1.1",
	})
	req1.Header.Set(tokenHeader, deployToken)
	res1 := performTestRequest(server.Handler, req1)
	assert.Equal(http.StatusOK, res1.Code)

	var body1 ResponseMessage
	err := json.Unmarshal(res1.Body.Bytes(), &body1)
	assert.NoError(err)
	assert.Equal("Redeployment triggered", body1.Message)

	time.Sleep(200 * time.Millisecond)
	assert.Equal("repository/svc:1.1", dc.PullArg)
	assert.Equal("test-svc", dc.GetImageIDArg)
	assert.Equal("test-svc", dc.RemoveContainerArg)
	assert.Equal("repository/svc:1.0", dc.RemoveImageArg)

	dc.Reset()
	dc.GetImageIDErr = errNoSuchContainer

	req2 := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "repository/svc:1.1",
	})
	req2.Header.Set(tokenHeader, deployToken)
	res2 := performTestRequest(server.Handler, req2)
	assert.Equal(http.StatusOK, res2.Code)

	var body2 ResponseMessage
	err = json.Unmarshal(res2.Body.Bytes(), &body2)
	assert.NoError(err)
	assert.Equal("Redeployment triggered", body2.Message)

	time.Sleep(200 * time.Millisecond)
	assert.Equal("repository/svc:1.1", dc.PullArg)
	assert.Equal("test-svc", dc.GetImageIDArg)
	assert.Equal("", dc.RemoveContainerArg)
	assert.Equal("", dc.RemoveImageArg)
}

func TestRedeploy_notOk(t *testing.T) {
	assert := assert.New(t)
	deployToken := "625181dbfb5c6100cdacd97f3ba32ab4"

	e := &env{
		cfg: Config{
			Authentication: AuthKey{
				Key:  "alg=scrypt$N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739",
				Salt: "478c1d403dec20707cf487f81c06d646",
			},
			Services: map[string]Target{
				"test-svc": Target{
					ID:        "test-svc",
					Binary:    "/bin/sh",
					Script:    "./resources/test-svc.sh",
					MustMatch: "^repository/svc:.*",
				},
			},
		},
	}
	server := newServer(e, 9000)

	reqMissing := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "missing",
		Image:  "repository/svc:1.1",
	})
	reqMissing.Header.Set(tokenHeader, deployToken)
	resMissing := performTestRequest(server.Handler, reqMissing)
	assert.Equal(http.StatusNotFound, resMissing.Code)

	reqUnauth := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "repository/svc:1.1",
	})
	reqUnauth.Header.Set(tokenHeader, "some-wrong-token")
	resUnauth := performTestRequest(server.Handler, reqUnauth)
	assert.Equal(http.StatusUnauthorized, resUnauth.Code)

	reqNoAuth := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "repository/svc:1.1",
	})
	resNoAuth := performTestRequest(server.Handler, reqNoAuth)
	assert.Equal(http.StatusUnauthorized, resNoAuth.Code)

	reqForbidden1 := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "repository/other:1.1",
	})
	reqForbidden1.Header.Set(tokenHeader, deployToken)
	resForbidden1 := performTestRequest(server.Handler, reqForbidden1)
	assert.Equal(http.StatusForbidden, resForbidden1.Code)

	reqForbidden2 := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "other/svc:1.1",
	})
	reqForbidden2.Header.Set(tokenHeader, deployToken)
	resForbidden2 := performTestRequest(server.Handler, reqForbidden2)
	assert.Equal(http.StatusForbidden, resForbidden2.Code)

	reqForbidden3 := createTestRequest("/redeploy", http.MethodPost, RedeploymentRequest{
		Target: "test-svc",
		Image:  "other-start-repository/other:1.1",
	})
	reqForbidden3.Header.Set(tokenHeader, deployToken)
	resForbidden3 := performTestRequest(server.Handler, reqForbidden3)
	assert.Equal(http.StatusForbidden, resForbidden3.Code)
}

func TestHealth(t *testing.T) {
	assert := assert.New(t)

	e := &env{
		cfg: Config{
			Authentication: AuthKey{
				Key:  "alg=scrypt$N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739",
				Salt: "478c1d403dec20707cf487f81c06d646",
			},
		},
	}
	server := newServer(e, 9000)

	req := createTestRequest("/health", http.MethodGet, nil)
	res := performTestRequest(server.Handler, req)
	assert.Equal(http.StatusOK, res.Code)

	var body ResponseMessage
	err := json.Unmarshal(res.Body.Bytes(), &body)
	assert.NoError(err)
	assert.Equal("OK", body.Message)
}

type mockDockerClient struct {
	PullArg string
	PullErr error

	GetImageIDArg    string
	GetImageIDOutput string
	GetImageIDErr    error

	RemoveContainerArg string
	RemoveContainerErr error

	RemoveImageArg string
	RemoveImageErr error
}

func (c *mockDockerClient) Pull(ctx *Context, image string) error {
	c.PullArg = image
	return c.PullErr
}

func (c *mockDockerClient) GetImageID(ctx *Context, name string) (string, error) {
	c.GetImageIDArg = name
	return c.GetImageIDOutput, c.GetImageIDErr
}

func (c *mockDockerClient) RemoveContainer(ctx *Context, name string) error {
	c.RemoveContainerArg = name
	return c.RemoveContainerErr
}

func (c *mockDockerClient) RemoveImage(ctx *Context, image string) error {
	c.RemoveImageArg = image
	return c.RemoveImageErr
}

func (c *mockDockerClient) Reset() {
	c.PullArg = ""
	c.PullErr = nil

	c.GetImageIDArg = ""
	c.GetImageIDOutput = ""
	c.GetImageIDErr = nil

	c.RemoveContainerArg = ""
	c.RemoveContainerErr = nil

	c.RemoveImageArg = ""
	c.RemoveImageErr = nil
}

func performTestRequest(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createTestRequest(route, method string, body interface{}) *http.Request {
	var reqBody io.Reader
	if body != nil {
		bytesBody, err := json.Marshal(body)
		if err != nil {
			log.Fatal("Failed to marshal body", zap.Error(err))
		}
		reqBody = bytes.NewBuffer(bytesBody)
	}

	req, err := http.NewRequest(method, route, reqBody)
	if err != nil {
		log.Fatal("Failed to create request", zap.Error(err))
	}

	return req
}
