package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/modules/database"
	"github.com/gin-gonic/gin"
)

func TestPipelineRequestShape(t *testing.T) {
	const stages = `[{"name":"Review","kind":"open"},{"name":"Won","kind":"won"},{"name":"Lost","kind":"lost"}]`
	create := `{"name":"Renewals","stages":` + stages + `}`
	input, err := pipelineInput([]byte(create), true)
	if err != nil || input.Name != "Renewals" || len(input.Stages) != 3 || input.Stages[2].Kind != "lost" {
		t.Fatal(input, err)
	}
	update := strings.Replace(create, `"stages"`, `"version":3,"stages"`, 1)
	update = strings.Replace(update, `"name":"Review"`, `"id":"`+strings.ToUpper(testWorkspace)+`","name":"Review"`, 1)
	input, err = pipelineInput([]byte(update), false)
	if err != nil || input.Version != 3 || input.Stages[0].ID == nil || *input.Stages[0].ID != testWorkspace {
		t.Fatal(input, err)
	}
	if _, err = pipelineInput([]byte(create), false); err == nil {
		t.Fatal("update accepted without version")
	}
	for _, raw := range []string{
		update, `{}`, `{"name":"x","stages":[]}`, `{"name":"x","stages":null}`,
		strings.Replace(create, `"name":"Review"`, `"name":"X","name":"Review"`, 1),
		strings.Replace(create, `"kind":"open"`, `"kind":null`, 1),
		strings.Replace(create, `"kind":"open"`, `"kind":"open","actor_id":"user"`, 1),
		strings.Replace(create, `"stages":`, `"is_default":true,"stages":`, 1),
		create + "{}",
	} {
		if _, err := pipelineInput([]byte(raw), true); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, version := range []string{"null", "0", "-1", "false", "1.5"} {
		if _, err := pipelineInput([]byte(strings.Replace(update, `"version":3`, `"version":`+version, 1)), false); err == nil {
			t.Errorf("accepted version %s", version)
		}
	}
}

func TestDealStageInput(t *testing.T) {
	body := []byte(`{"stage_id":"` + strings.ToUpper(testWorkspace) + `"}`)
	convert, err := conversionInput(body)
	if err != nil || convert.StageID == nil || *convert.StageID != testWorkspace {
		t.Fatal(convert, err)
	}
	patch, err := dealUpdateInput(body)
	if err != nil || patch.StageID == nil || *patch.StageID != testWorkspace {
		t.Fatal(patch, err)
	}
	for _, raw := range []string{`{"stage_id":null}`, `{"stage_id":""}`, `{"stage_id":42}`, `{"stage_id":"bad"}`, `{"status":"won"}`} {
		if _, err := conversionInput([]byte(raw)); err == nil {
			t.Errorf("conversion accepted %s", raw)
		}
		if _, err := dealUpdateInput([]byte(raw)); err == nil {
			t.Errorf("PATCH accepted %s", raw)
		}
	}
}

func TestPipelineWriteRequiresWorkspaceAdmin(t *testing.T) {
	for _, method := range []string{"POST", "PUT"} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest(method, "/", nil)
		c.Set("crm_scope", database.Scope{All: true, Admin: false})
		(&handlers{}).writePipeline(c)
		if out.Code != 403 {
			t.Fatalf("%s: got %d", method, out.Code)
		}
	}
}
