package main_test

import (
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateRabbitService(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	service, err := tcr.NewRabbitService(Seasoning, "", "", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	service.Shutdown(true)
}

func TestCreateRabbitServiceWithEncryption(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	service.Shutdown(true)
}

func TestRabbitServicePublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	data := tcr.RandomBytes(1000)
	service.Publish(data, "", "TcrTestQueue", "", false, nil)

	service.Shutdown(true)
}

func TestRabbitServicePublishLetter(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	service.PublishLetter(letter)

	service.Shutdown(true)
}

func TestRabbitServicePublishAndConsumeLetter(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	service.PublishLetter(letter)

	service.Shutdown(true)
}
