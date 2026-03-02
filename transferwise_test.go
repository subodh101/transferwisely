package main

import (
	"bytes"
	"encoding/json"
	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"transferwisely/mocks"
)

func init() {
	Client = &mocks.Client{}
}

func TestGetTransfer(t *testing.T) {
	t.Run("success - multiple transfers", func(t *testing.T) {
		// Mock a list of 2 transfers
		transfers := []Transfer{
			{Id: 101, SourceCurrency: "JPY", TargetCurrency: "INR", Rate: 0.58},
			{Id: 102, SourceCurrency: "JPY", TargetCurrency: "INR", Rate: 0.57},
		}
		j, _ := json.Marshal(transfers)
		r := ioutil.NopCloser(bytes.NewReader(j))

		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		res, err := getTransfer()
		assert.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, uint64(101), res[0].Id)
	})
}

func TestGetDetailByQuoteId(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := QuoteDetail{}
		_ = faker.FakeData(&q)
		j, _ := json.Marshal(q)
		r := ioutil.NopCloser(bytes.NewReader(j))

		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		qd, err := getDetailByQuoteId("quote-uuid")
		assert.NoError(t, err)
		assert.Equal(t, q.Id, qd.Id)
	})
}

func TestGenerateQuote(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := QuoteDetail{Id: "new-quote-123"}
		j, _ := json.Marshal(q)
		r := ioutil.NopCloser(bytes.NewReader(j))

		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		// Updated to match your 4-argument signature: (source, target, amount, profile)
		qId, err := generateQuote("JPY", "INR", 50000.00, 12345)
		assert.NoError(t, err)
		assert.Equal(t, "new-quote-123", qId)
	})
}

func TestCancelTransfer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{}`))),
			}, nil
		}

		result, err := cancelTransfer(1)
		assert.True(t, result)
		assert.NoError(t, err)
	})

	t.Run("error - API failure", func(t *testing.T) {
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"error":"failed"}`))),
			}, nil
		}

		result, err := cancelTransfer(1)
		assert.False(t, result)
		assert.Error(t, err)
	})
}
