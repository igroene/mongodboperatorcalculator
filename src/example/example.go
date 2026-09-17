package main

import (
	"fmt"

	MO "github.com/igroene/mongodboperatorcalculator/src/mongodboperatorcalculator"
)

func main() {
	var conf MO.Configuration
	conf.Init()

	request := MO.ConfigurationRequest{
		Output:         MO.ResultOutputFormatJson,
		DBType:         MO.DbTypeShardedCluster,
		Dimension:      MO.Dimension{Id: 3},
		LoadType:       MO.LoadType{Id: MO.LoadTypeSomeWrites},
		Connections:    500,
		MongoDBVersion: MO.Version{Major: 7, Minor: 0, Patch: 0},
	}

	var calculator MO.MongoDBOperatorCalculator
	request = calculator.Init(request, conf)
	err, message, families := calculator.GetCalculate()
	if err != nil {
		fmt.Printf("calculation failed: %v\n", err)
		return
	}
	output, err := calculator.GetJSONOutput(message, request, families)
	if err != nil {
		fmt.Printf("output failed: %v\n", err)
		return
	}
	fmt.Println(output.String())
}
