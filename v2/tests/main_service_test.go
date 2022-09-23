package main_test

import (
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateRabbitService(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	service, err := tcr.NewRabbitService(cfg.Seasoning, "", "", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	service.Shutdown(true)
}

func TestCreateRabbitServiceWithEncryption(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	service.Shutdown(true)
}

func TestRabbitServicePublish(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	data := tcr.RandomBytes(1000)
	_ = service.Publish(data, "", "TcrTestQueue", "", false, nil)

	service.Shutdown(true)
}

func TestRabbitServicePublishLetter(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	_ = service.PublishLetter(letter)

	service.Shutdown(true)
}

func TestRabbitServicePublishAndConsumeLetter(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	_ = service.PublishLetter(letter)

	service.Shutdown(true)
}

func TestRabbitServicePublishLetterToNonExistentQueueForRetryTesting(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.PublisherConfig.PublishTimeOutInterval = 0 // triggering instant timeouts for retry test
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	letter := tcr.CreateMockRandomLetter("QueueDoesNotExist")
	_ = service.QueueLetter(letter)

	timeout := time.After(time.Duration(2 * time.Second))
	<-timeout

	service.Shutdown(true)
}
