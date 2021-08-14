package zfs

import (
	"strings"
)

// DatasetKind enum of supported dataset types
type DatasetKind string

const (
	// DatasetFilesystem enum entry
	DatasetFilesystem DatasetKind = `filesystem`
	// DatasetVolume enum entry
	DatasetVolume DatasetKind = `volume`
	// DatasetSnapshot enum entry
	DatasetSnapshot DatasetKind = `snapshot`
)

// Dataset holds the properties for an individual dataset
type Dataset struct {
	Pool       string
	Name       string
	Properties map[string]string
}

// DatasetProperties returns the requested properties for all datasets in the given pool
func DatasetProperties(pool string, kind DatasetKind, properties ...string) ([]Dataset, error) {
	handler := newDatasetHandler()
	if err := execute(pool, handler, `zfs`, `get`, `-Hprt`, string(kind), `-o`, `name,property,value`, strings.Join(properties, `,`)); err != nil {
		return nil, err
	}
	return handler.datasets(), nil
}

// datasetHandler handles parsing of the data returned from the CLI into Dataset structs
type datasetHandler struct {
	store map[string]Dataset
}

// processLine implements the handler interface
func (h *datasetHandler) processLine(pool string, line []string) error {
	if len(line) != 3 {
		return ErrInvalidOutput
	}
	if _, ok := h.store[line[0]]; !ok {
		h.store[line[0]] = newDataset(pool, line[0])
	}
	h.store[line[0]].Properties[line[1]] = line[2]
	return nil
}

func (h *datasetHandler) datasets() []Dataset {
	result := make([]Dataset, len(h.store))
	i := 0
	for _, dataset := range h.store {
		result[i] = dataset
		i++
	}
	return result
}

func newDataset(pool string, name string) Dataset {
	return Dataset{
		Pool:       pool,
		Name:       name,
		Properties: make(map[string]string),
	}
}

func newDatasetHandler() *datasetHandler {
	return &datasetHandler{store: make(map[string]Dataset)}
}
