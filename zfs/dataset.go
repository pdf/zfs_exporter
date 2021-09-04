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

type datasetsImpl struct {
	pool string
	kind DatasetKind
}

func (d datasetsImpl) Pool() string {
	return d.pool
}

func (d datasetsImpl) Kind() DatasetKind {
	return d.kind
}

func (d datasetsImpl) Properties(props ...string) ([]DatasetProperties, error) {
	handler := newDatasetHandler()
	if err := execute(d.pool, handler, `zfs`, `get`, `-Hprt`, string(d.kind), `-o`, `name,property,value`, strings.Join(props, `,`)); err != nil {
		return nil, err
	}
	return handler.datasets(), nil
}

type datasetPropertiesImpl struct {
	datasetName string
	properties  map[string]string
}

func (p *datasetPropertiesImpl) DatasetName() string {
	return p.datasetName
}

func (p *datasetPropertiesImpl) Properties() map[string]string {
	return p.properties
}

// datasetHandler handles parsing of the data returned from the CLI into Dataset structs
type datasetHandler struct {
	store map[string]*datasetPropertiesImpl
}

// processLine implements the handler interface
func (h *datasetHandler) processLine(pool string, line []string) error {
	if len(line) != 3 || !strings.HasPrefix(line[0], pool) {
		return ErrInvalidOutput
	}
	if _, ok := h.store[line[0]]; !ok {
		h.store[line[0]] = newDatasetPropertiesImpl(line[0])
	}
	h.store[line[0]].properties[line[1]] = line[2]
	return nil
}

func (h *datasetHandler) datasets() []DatasetProperties {
	result := make([]DatasetProperties, len(h.store))
	i := 0
	for _, dataset := range h.store {
		result[i] = dataset
		i++
	}
	return result
}

func newDatasetPropertiesImpl(name string) *datasetPropertiesImpl {
	return &datasetPropertiesImpl{
		datasetName: name,
		properties:  make(map[string]string),
	}
}

func newDatasetsImpl(pool string, kind DatasetKind) datasetsImpl {
	return datasetsImpl{
		pool: pool,
		kind: kind,
	}
}

func newDatasetHandler() *datasetHandler {
	return &datasetHandler{
		store: make(map[string]*datasetPropertiesImpl),
	}
}
