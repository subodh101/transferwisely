package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"transferwisely/mocks"
)

func init() {
	Client = &mocks.Client{}
}

func TestGetDetailByQuoteId(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// build response JSON
		q := QuoteDetail{}
		err := faker.FakeData(&q)
		if err != nil {
			log.Println(err)
		}
		j, _ := json.Marshal(q)
		// create a new reader with that JSON
		r := ioutil.NopCloser(bytes.NewReader(j))
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		qd, err := getDetailByQuoteId("anything")
		assert.NotEmpty(t, qd)
		assert.Equal(t, q.Id, qd.Id)
		assert.NoError(t, err)
	})

	t.Run("external api error", func(t *testing.T) {
		// build response JSON
		q := QuoteDetail{}
		j, _ := json.Marshal(q)
		// create a new reader with that JSON
		r := ioutil.NopCloser(bytes.NewReader(j))
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       r,
			}, nil
		}

		qd, err := getDetailByQuoteId("anything")
		assert.Empty(t, qd)
		assert.Error(t, err)

	})
}

func TestGenerateQuote(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := QuoteDetail{
			Id:           "123-abc",
			TargetAmount: 25000.00,
			SourceAmount: 43000.00,
		}
		j, _ := json.Marshal(q)
		r := ioutil.NopCloser(bytes.NewReader(j))

		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		// Pass the 4 required arguments
		// generateQuote(source, target, targetAmount, profile)
		qId, err := generateQuote("JPY", "INR", 25000.00, 1)

		assert.NoError(t, err)
		assert.Equal(t, "123-abc", qId)
	})
}

func TestCancelTransfer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// build response JSON
		tId := uint64(1)
		j, _ := json.Marshal(tId)
		// create a new reader with that JSON
		r := ioutil.NopCloser(bytes.NewReader(j))
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       r,
			}, nil
		}

		result, err := cancelTransfer(tId)
		assert.Equal(t, result, true)
		assert.NoError(t, err)
	})

	t.Run("external api error", func(t *testing.T) {
		// build response JSON
		err := errors.New("error calling external API")
		j, _ := json.Marshal(err)
		// create a new reader with that JSON
		r := ioutil.NopCloser(bytes.NewReader(j))
		mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       r,
			}, nil
		}

		result, err := cancelTransfer(1)
		assert.Equal(t, result, false)
		assert.Error(t, err)
	})
}
