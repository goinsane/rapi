package rapi

import "strings"

type HeaderOption struct {
	KeyVals []HeaderOptionKeyVal
	Map     map[string]string
}

type HeaderOptionKeyVal struct {
	Key string
	Val string
}

func ParseHeaderOptions(directive string) (options []HeaderOption) {
	options = []HeaderOption{}

	for _, o := range strings.Split(directive, ",") {
		o = strings.TrimSpace(o)
		option := &HeaderOption{
			KeyVals: []HeaderOptionKeyVal{},
			Map:     map[string]string{},
		}
		for _, kv := range strings.Split(o, ";") {
			kv = strings.TrimSpace(kv)
			kvs := strings.SplitN(kv, "=", 2)
			optionKeyVal := &HeaderOptionKeyVal{
				Key: strings.TrimSpace(kvs[0]),
			}
			if optionKeyVal.Key == "" {
				continue
			}
			if len(kvs) > 1 {
				optionKeyVal.Val = strings.TrimSpace(kvs[1])
			}
			option.KeyVals = append(option.KeyVals, *optionKeyVal)
			if _, ok := option.Map[optionKeyVal.Key]; !ok {
				option.Map[optionKeyVal.Key] = optionKeyVal.Val
			}
		}
		if len(option.KeyVals) <= 0 {
			continue
		}
		options = append(options, *option)
	}

	return
}
