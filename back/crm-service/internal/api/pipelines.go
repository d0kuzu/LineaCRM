package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listPipelines(c *gin.Context) {
	rows, err := h.db.ListPipelines(c.Request.Context(), scope(c))
	recordResponse(c, http.StatusOK, rows, err)
}

func (h *handlers) writePipeline(c *gin.Context) {
	s := scope(c)
	if !s.Admin {
		c.JSON(http.StatusForbidden, gin.H{"error": database.ErrPipelineForbidden.Error()})
		return
	}
	body, ok := readBody(c, 64<<10)
	if !ok {
		return
	}
	create := c.Request.Method == http.MethodPost
	input, err := pipelineInput(body, create)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if create {
		pipeline, err := h.db.CreatePipeline(c.Request.Context(), s, input)
		recordResponse(c, http.StatusCreated, pipeline, err)
		return
	}
	pipeline, err := h.db.UpdatePipeline(c.Request.Context(), s, recordID(c), input)
	recordResponse(c, http.StatusOK, pipeline, err)
}

func stageID(decoder *json.Decoder) (*string, error) {
	var id string
	if decoder.Decode(&id) != nil || !validID(id) {
		return nil, errors.New("stage ID must be a UUID")
	}
	id = strings.ToLower(id)
	return &id, nil
}

func pipelineInput(body []byte, create bool) (database.PipelineInput, error) {
	var input database.PipelineInput
	err := decodeFieldsObject(body, false, func(name string, decoder *json.Decoder) error {
		switch name {
		case "name":
			if err := decoder.Decode(&input.Name); err != nil {
				return errors.New("pipeline name must be a string")
			}
		case "version":
			if create || decoder.Decode(&input.Version) != nil || input.Version < 1 {
				return errors.New("a positive version is required only when updating a pipeline")
			}
		case "stages":
			var stages []json.RawMessage
			if err := decoder.Decode(&stages); err != nil || len(stages) == 0 {
				return errors.New("stages must be a nonempty array")
			}
			for _, raw := range stages {
				stage, err := pipelineStageInput(raw, create)
				if err != nil {
					return err
				}
				input.Stages = append(input.Stages, stage)
			}
		default:
			return errors.New("unknown pipeline field")
		}
		return nil
	})
	if err == nil && (input.Name == "" || len(input.Stages) == 0 || (!create && input.Version < 1)) {
		err = errors.New("name and stages are required, plus the current version for updates")
	}
	return input, err
}

func pipelineStageInput(raw json.RawMessage, create bool) (database.PipelineStageInput, error) {
	var stage database.PipelineStageInput
	err := decodeFieldsObject(raw, false, func(name string, decoder *json.Decoder) error {
		switch name {
		case "id":
			if create {
				return errors.New("new stages must omit the ID")
			}
			id, err := stageID(decoder)
			stage.ID = id
			return err
		case "name":
			if decoder.Decode(&stage.Name) != nil {
				return errors.New("stage name must be a string")
			}
		case "kind":
			if decoder.Decode(&stage.Kind) != nil {
				return errors.New("stage kind must be a string")
			}
		default:
			return errors.New("unknown stage field")
		}
		return nil
	})
	if err == nil && (stage.Name == "" || stage.Kind == "") {
		err = errors.New("each stage requires a name and kind")
	}
	return stage, err
}
