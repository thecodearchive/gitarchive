package metrics

import (
	"encoding/json"
	"expvar"
	"log"
	"time"

	"github.com/influxdata/influxdb/client/v2"
)

func StartInfluxExport(addr, table string, v *expvar.Map) error {
	c, err := client.NewHTTPClient(client.HTTPConfig{Addr: addr})
	if err != nil {
		return err
	}

	q := client.NewQuery("CREATE DATABASE IF NOT EXISTS gitarchive", "", "")
	if response, err := c.Query(q); err != nil {
		return err
	} else if response.Error() != nil {
		return response.Error()
	}

	go func(c client.Client) {
		for range time.Tick(5 * time.Second) {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(v.String()), &fields); err != nil {
				log.Println("[-] InfluxDB json error: ", err.Error())
			}
			for name, val := range fields {
				if val, ok := val.(map[string]interface{}); ok {
					for innerName, innerVal := range val {
						fields[name+"."+innerName] = innerVal
					}
					delete(fields, name)
				}
			}

			if len(fields) == 0 {
				continue
			}

			bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
				Database:  "gitarchive",
				Precision: "s",
			})
			pt, err := client.NewPoint(table, nil, fields, time.Now())
			if err != nil {
				log.Println("[-] InfluxDB error: ", err.Error())
			}
			bp.AddPoint(pt)
			if err := c.Write(bp); err != nil {
				log.Println("[-] InfluxDB write error: ", err.Error())
			}
		}
	}(c)

	return nil
}
