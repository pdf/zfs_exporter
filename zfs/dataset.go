package zfs

import (
	"errors"
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

// Keystatus enum contains keystatus string value
type Keystatus string

const (
	KeystatusNone        Keystatus = `none`
	KeystatusAvailable   Keystatus = `available`
	KeystatusUnavailable Keystatus = `unavailable`
)

type datasetsImpl struct {
	run                runner
	pool               string
	kind               DatasetKind
	excludeDescendants []string
}

func (d datasetsImpl) Pool() string {
	return d.pool
}

func (d datasetsImpl) Kind() DatasetKind {
	return d.kind
}

func (d datasetsImpl) Properties(props ...string) ([]DatasetProperties, error) {
	prune := descendantsIn(d.pool, d.excludeDescendants)
	if len(prune) == 0 {
		handler := newDatasetHandler()
		if err := execute(d.run, d.pool, []string{d.pool}, handler, `zfs`, `get`, `-Hprt`, string(d.kind), `-o`, `name,property,value`, strings.Join(props, `,`)); err != nil {
			return nil, err
		}
		return handler.datasets(), nil
	}

	// Datasets may be destroyed between planning and querying, so a missing
	// dataset gets one fresh plan.
	handler, err := d.prunedProperties(prune, props)
	if errors.Is(err, errDatasetNotExist) {
		handler, err = d.prunedProperties(prune, props)
	}
	if err != nil {
		return nil, err
	}
	return handler.datasets(), nil
}

// prunedProperties queries the datasets of the pool without visiting anything
// below a dataset in prune.
func (d datasetsImpl) prunedProperties(prune []string, props []string) (*datasetHandler, error) {
	recursive, shallow, err := planTargets(d.pool, prune, func(node string) ([]string, error) {
		return listChildren(d.run, node)
	})
	if err != nil {
		return nil, err
	}

	handler := newDatasetHandler()
	propList := strings.Join(props, `,`)
	if len(recursive) > 0 {
		if err := execute(d.run, d.pool, recursive, handler, `zfs`, `get`, `-Hprt`, string(d.kind), `-o`, `name,property,value`, propList); err != nil {
			return nil, err
		}
	}
	if len(shallow) > 0 {
		// A snapshot is one level below its dataset.
		depth := `0`
		if d.kind == DatasetSnapshot {
			depth = `1`
		}
		if err := execute(d.run, d.pool, shallow, handler, `zfs`, `get`, `-Hpt`, string(d.kind), `-d`, depth, `-o`, `name,property,value`, propList); err != nil {
			return nil, err
		}
	}
	return handler, nil
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

func newDatasetsImpl(run runner, pool string, kind DatasetKind, excludeDescendants []string) datasetsImpl {
	return datasetsImpl{
		run:                run,
		pool:               pool,
		kind:               kind,
		excludeDescendants: excludeDescendants,
	}
}

func newDatasetHandler() *datasetHandler {
	return &datasetHandler{
		store: make(map[string]*datasetPropertiesImpl),
	}
}
