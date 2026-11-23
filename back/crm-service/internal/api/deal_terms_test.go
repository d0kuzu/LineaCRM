package api

import (
	"testing"

	"crm-service/internal/modules/database"
)

func TestDealTermInputs(t *testing.T) {
	decoders := map[string]func([]byte) (database.DealTermsPatch, error){
		"convert": func(body []byte) (database.DealTermsPatch, error) {
			input, err := conversionInput(body)
			return input.DealTermsPatch, err
		},
		"patch": func(body []byte) (database.DealTermsPatch, error) {
			input, err := dealUpdateInput(body)
			return input.DealTermsPatch, err
		},
	}
	for name, decode := range decoders {
		t.Run(name, func(t *testing.T) {
			terms, err := decode([]byte(`{"amount":"99999999999999.9999","currency":"kzt","expected_close_date":"2028-02-29","probability":0}`))
			if err != nil {
				t.Fatal(err)
			}
			if terms.Amount.Value == nil || *terms.Amount.Value != "99999999999999.9999" ||
				terms.Currency.Value == nil || *terms.Currency.Value != "KZT" ||
				terms.ExpectedCloseDate.Value == nil || *terms.ExpectedCloseDate.Value != "2028-02-29" ||
				terms.Probability.Value == nil || *terms.Probability.Value != 0 {
				t.Fatalf("unexpected deal values: %+v", terms)
			}
			terms, err = decode([]byte(`{"expected_close_date":null,"probability":100}`))
			if err != nil || terms.Amount.Present || terms.Currency.Present ||
				!terms.ExpectedCloseDate.Present || terms.ExpectedCloseDate.Value != nil ||
				!terms.Probability.Present || terms.Probability.Value == nil || *terms.Probability.Value != 100 {
				t.Fatalf("unexpected partial update: %+v, %v", terms, err)
			}
			for _, raw := range []string{
				`{"amount":1.25}`, `{"amount":"100000000000000"}`, `{"amount":"12.12345"}`,
				`{"amount":"-1"}`, `{"amount":"1e2"}`, `{"amount":"1,50"}`,
				`{"currency":"ZZZ"}`, `{"currency":"KZ"}`, `{"currency":5}`,
				`{"expected_close_date":"2027-02-29"}`, `{"expected_close_date":"0000-01-01"}`,
				`{"expected_close_date":"2028-02-29T00:00:00Z"}`,
				`{"probability":101}`, `{"probability":-1}`, `{"probability":"25"}`, `{"probability":0.5}`,
				`{"probability":0,"probability":null}`,
			} {
				if _, err := decode([]byte(raw)); err == nil {
					t.Errorf("accepted invalid deal values %s", raw)
				}
			}
		})
	}
}
