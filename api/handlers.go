package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/rpaas"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func scale(c echo.Context) error {
	qty := c.FormValue("quantity")
	if len(qty) == 0 {
		return c.String(http.StatusBadRequest, "missing quantity")
	}
	intQty, err := strconv.Atoi(qty)
	if err != nil || intQty <= 0 {
		return c.String(http.StatusBadRequest, "invalid quantity: "+qty)
	}

	name := c.Param("instance")
	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: NAMESPACE,
		},
	}
	ctx := context.TODO()
	err = cli.Get(ctx, types.NamespacedName{Name: name, Namespace: NAMESPACE}, instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.NoContent(http.StatusNotFound)
		}
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}

	int32Qty := int32(intQty)
	instance.Spec.Replicas = &int32Qty
	err = cli.Update(ctx, instance)
	if err != nil {
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusCreated)
}

func updateCertificate(c echo.Context) error {
	rawCertificate, err := getFormFileContent(c, "cert")
	if err != nil {
		if err == http.ErrMissingFile {
			return c.String(http.StatusBadRequest, "cert file is either not provided or not valid")
		}
		return err
	}
	rawKey, err := getFormFileContent(c, "key")
	if err != nil {
		if err == http.ErrMissingFile {
			return c.String(http.StatusBadRequest, "key file is either not provided or not valid")
		}
		return err
	}
	certificate, err := tls.X509KeyPair(rawCertificate, rawKey)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("could not load the given certicate and key: %s", err))
	}
	manager, ok := c.Get("manager").(rpaas.RpaasManager)
	if !ok {
		return errors.New("invalid manager state")
	}
	instance := c.Param("instance")
	err = manager.UpdateCertificate(instance, certificate)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func getFormFileContent(c echo.Context, key string) ([]byte, error) {
	fileHeader, err := c.FormFile(key)
	if err != nil {
		return []byte{}, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()
	rawContent, err := ioutil.ReadAll(file)
	if err != nil {
		return []byte{}, err
	}
	return rawContent, nil
}
