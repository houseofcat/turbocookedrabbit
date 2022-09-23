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

	service.Close()
}

func TestCreateRabbitServiceWithEncryption(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.EncryptionConfig.Enabled = true
	service, err := tcr.NewRabbitService(cfg.Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	service.Close()
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

	service.Close()
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

	service.Close()
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

	service.Close()
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

	service.Close()
}
