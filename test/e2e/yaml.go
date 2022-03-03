package e2e

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

func yamlContainKeyValue(yamlNodes []*yaml.Node, value string, keys ...string) ([]*yaml.Node, error) {
	if yamlNodes == nil {
		return nil, errors.New("input list of yaml node is null")
	}
	foundNode := []*yaml.Node{}
	for _, obj := range yamlNodes {
		if obj.Kind == yaml.DocumentNode {
			obj = obj.Content[0] // We can ignore the document node and focus on the block-mapping node.
		}
		field, err := yamlFindByValue(obj, keys...)
		if err == nil && field.Value == value {
			foundNode = append(foundNode, obj)
		}
	}
	if len(foundNode) == 0 {
		return nil, errors.New("could not find the appropriate yaml node")
	}
	return foundNode, nil
}

func yamlFindByValue(node *yaml.Node, values ...string) (*yaml.Node, error) {
	if node == nil {
		return nil, errors.New("input yaml node is null")
	}
	value := values[0]
	for i, child := range node.Content {
		if child.Value == value {
			targetNode := node.Content[i+1]
			if len(values[1:]) > 0 {
				return yamlFindByValue(targetNode, values[1:]...)
			}
			return targetNode, nil
		}
	}
	return nil, errors.New("could not find the appropriate yaml node")
}

func splitYAML(resources []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(resources))
	listDocument := []*yaml.Node{}
	for {
		var value yaml.Node
		err := dec.Decode(&value)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		listDocument = append(listDocument, &value)
	}

	return listDocument, nil
}

func printYaml(listDocument []*yaml.Node) ([]byte, error) {
	var out []byte
	for _, doc := range listDocument {
		marshalDoc, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		out = append(append(out, []byte("\n---\n")...), marshalDoc...)
	}
	return out, nil
}
