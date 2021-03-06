package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupTest(t *testing.T) {
	scheme, err := v1alpha1.SchemeBuilder.Build()
	require.Nil(t, err)
	cli = fake.NewFakeClientWithScheme(scheme)

	err = cli.Create(context.TODO(), &v1alpha1.RpaasPlan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasPlan",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myplan",
			Namespace: NAMESPACE,
		},
	})
	require.Nil(t, err)
	err = cli.Create(context.TODO(), &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firstinstance",
			Namespace: NAMESPACE,
		},
	})
	require.Nil(t, err)
}

func Test_serviceCreate(t *testing.T) {
	setupTest(t)

	testCases := []struct {
		requestBody  string
		expectedCode int
		expectedBody string
	}{
		{
			"",
			http.StatusBadRequest,
			"name is required",
		},
		{
			"name=rpaas",
			http.StatusBadRequest,
			"plan is required",
		},
		{
			"name=rpaas&plan=myplan",
			http.StatusBadRequest,
			"team name is required",
		},
		{
			"name=rpaas&plan=plan2&team=myteam",
			http.StatusBadRequest,
			"invalid plan",
		},
		{
			"name=firstinstance&plan=myplan&team=myteam",
			http.StatusConflict,
			"firstinstance instance already exists",
		},
		{
			"name=otherinstance&plan=myplan&team=myteam",
			http.StatusCreated,
			"",
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("when body == %q", testCase.requestBody), func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(http.MethodPost, "/resources", strings.NewReader(testCase.requestBody))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			recorder := httptest.NewRecorder()
			context := e.NewContext(request, recorder)
			err := serviceCreate(context)
			assert.Nil(t, err)
			e.HTTPErrorHandler(err, context)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, testCase.expectedBody, recorder.Body.String())
		})
	}
}

func Test_serviceDelete(t *testing.T) {
	setupTest(t)

	testCases := []struct {
		instanceName string
		expectedCode int
		expectedBody string
	}{
		{
			"",
			http.StatusBadRequest,
			"name is required",
		},
		{
			"unknown",
			http.StatusNotFound,
			"",
		},
		{
			"firstinstance",
			http.StatusOK,
			"",
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("when instance name == %q", testCase.instanceName), func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(http.MethodDelete, "/resources/"+testCase.instanceName, nil)
			recorder := httptest.NewRecorder()
			context := e.NewContext(request, recorder)
			context.SetParamNames("instance")
			context.SetParamValues(testCase.instanceName)
			err := serviceDelete(context)
			assert.Nil(t, err)
			e.HTTPErrorHandler(err, context)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, testCase.expectedBody, recorder.Body.String())
		})
	}
}

func Test_servicePlans(t *testing.T) {
	setupTest(t)

	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/resources/plans", nil)
	recorder := httptest.NewRecorder()
	context := e.NewContext(request, recorder)
	err := servicePlans(context)
	assert.Nil(t, err)
	e.HTTPErrorHandler(err, context)
	assert.Equal(t, http.StatusOK, recorder.Code)

	type result struct {
		Name, Description string
	}
	r := []result{}
	err = json.Unmarshal(recorder.Body.Bytes(), &r)
	require.Nil(t, err)
	expected := []result{{Name: "myplan", Description: "no plan description"}}
	assert.Equal(t, expected, r)
}

func Test_serviceInfo(t *testing.T) {
	setupTest(t)
	replicas := int32(3)
	err := cli.Update(context.TODO(), &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firstinstance",
			Namespace: NAMESPACE,
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Replicas: &replicas,
			Locations: []v1alpha1.Location{
				{Config: v1alpha1.ConfigRef{Value: "/status"}},
				{Config: v1alpha1.ConfigRef{Value: "/admin"}},
			},
		},
	})
	require.Nil(t, err)

	testCases := []struct {
		instanceName     string
		expectedCode     int
		expectedReplicas string
		expectedRoutes   string
	}{
		{
			"",
			http.StatusBadRequest,
			"",
			"",
		},
		{
			"unknown",
			http.StatusNotFound,
			"",
			"",
		},
		{
			"firstinstance",
			http.StatusOK,
			"3",
			"/status\n/admin",
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("when instance name == %q", testCase.instanceName), func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(http.MethodGet, "/resources/"+testCase.instanceName, nil)
			recorder := httptest.NewRecorder()
			context := e.NewContext(request, recorder)
			context.SetParamNames("instance")
			context.SetParamValues(testCase.instanceName)
			err := serviceInfo(context)
			assert.Nil(t, err)
			e.HTTPErrorHandler(err, context)
			require.Equal(t, testCase.expectedCode, recorder.Code)

			if recorder.Code == http.StatusOK {
				var r []map[string]string
				err = json.Unmarshal(recorder.Body.Bytes(), &r)
				require.Nil(t, err)
				expected := []map[string]string{
					{
						"label": "Address",
						"value": "x.x.x.x",
					},
					{
						"label": "Instances",
						"value": testCase.expectedReplicas,
					},
					{
						"label": "Routes",
						"value": testCase.expectedRoutes,
					},
				}
				assert.Equal(t, expected, r)
			}
		})
	}
}

func Test_serviceBindApp(t *testing.T) {
	setupTest(t)

	testCases := []struct {
		instanceName string
		expectedCode int
		appName      string
		appHost      string
		eventId      string
	}{
		{
			"",
			http.StatusBadRequest,
			"",
			"",
			"",
		},
		{
			"unknown",
			http.StatusNotFound,
			"",
			"",
			"",
		},
		{
			"firstinstance",
			http.StatusCreated,
			"myapp",
			"myapp.example.com",
			"12345",
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("when instance name == %q", testCase.instanceName), func(t *testing.T) {
			e := echo.New()
			body := fmt.Sprintf("app-name=%s&app-host=%s&eventid=%s", testCase.appName, testCase.appHost, testCase.eventId)
			request := httptest.NewRequest(http.MethodPost, "/resources/"+testCase.instanceName+"/bind-app", strings.NewReader(body))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(request, recorder)
			ctx.SetParamNames("instance", "app-name", "app-host", "eventid")
			ctx.SetParamValues(testCase.instanceName, testCase.appName, testCase.appHost, testCase.eventId)
			err := serviceBindApp(ctx)
			require.Nil(t, err)
			e.HTTPErrorHandler(err, ctx)
			require.Equal(t, testCase.expectedCode, recorder.Code)

			if recorder.Code == http.StatusCreated {
				instance := &v1alpha1.RpaasBind{}
				err = cli.Get(context.TODO(), types.NamespacedName{Name: testCase.instanceName, Namespace: NAMESPACE}, instance)
				require.Nil(t, err)
				expected := map[string]string{
					"app-name": testCase.appName,
					"app-host": testCase.appHost,
					"eventid":  testCase.eventId,
				}
				assert.Equal(t, expected, instance.ObjectMeta.Annotations)
			}
		})
	}
}

func Test_serviceUnbindApp(t *testing.T) {
	setupTest(t)
	err := cli.Create(context.TODO(), &v1alpha1.RpaasBind{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasBind",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mybind",
			Namespace: NAMESPACE,
		},
	})
	require.Nil(t, err)

	testCases := []struct {
		instanceName string
		expectedCode int
	}{
		{
			"",
			http.StatusBadRequest,
		},
		{
			"unknown",
			http.StatusNotFound,
		},
		{
			"mybind",
			http.StatusOK,
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("when instance name == %q", testCase.instanceName), func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(http.MethodDelete, "/resources/"+testCase.instanceName+"/bind-app", nil)
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(request, recorder)
			ctx.SetParamNames("instance")
			ctx.SetParamValues(testCase.instanceName)
			err := serviceUnbindApp(ctx)
			require.Nil(t, err)
			e.HTTPErrorHandler(err, ctx)
			require.Equal(t, testCase.expectedCode, recorder.Code)
		})
	}
}
