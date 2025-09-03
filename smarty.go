package main

import (
	"context"
	"errors"
	"log"

	street "github.com/smartystreets/smartystreets-go-sdk/us-street-api"
)

var ErrUnknownAddress = errors.New("unknown address")

func SmartyInfo(client *street.Client, addr *Address) error {
	lookup := &street.Lookup{
		Street:        addr.Street,
		City:          addr.City,
		State:         addr.State,
		ZIPCode:       addr.Zip,
		MaxCandidates: 1,
	}

	batch := street.NewBatch()
	batch.Append(lookup)

	if err := client.SendBatchWithContext(context.Background(), batch); err != nil {
		log.Println("发送请求失败: ", err)
		return err
	}

	for _, input := range batch.Records() {
		if len(input.Results) == 0 {
			log.Println("未找到匹配的地址: ", addr.Street, addr.City, addr.State, addr.Zip)
			return ErrUnknownAddress
		}

		candidate := input.Results[0]
		addr.CMRA = candidate.Analysis.DPVCMRACode
		addr.RDI = candidate.Metadata.RDI
	}

	return nil

}
