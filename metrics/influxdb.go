package metrics

import (
	"expvar"
	"log"
	"strconv"
	"time"

	"github.com/influxdata/influxdb/client/v2"
)

func StartInfluxExport(addr, table string, v *expvar.Map) error {
	c, err := client.NewHTTPClient(client.HTTPConfig{Addr: addr})
	if err != nil {
		return err
	}

	q := client.NewQuery("CREATE DATABASE IF NOT EXISTS gitarchive42", "", "")
	if response, err := c.Query(q); err != nil {
		return err
	} else if response.Error() != nil {
		return response.Error()
	}

	go func(c client.Client) {
		for range time.Tick(5 * time.Second) {
			fields := make(map[string]interface{})
			var do func(string, expvar.KeyValue)
			do = func(prefix string, kv expvar.KeyValue) {
				switch v := kv.Value.(type) {
				case *expvar.Int:
					x, _ := strconv.ParseInt(v.String(), 10, 0)
					fields[prefix+kv.Key] = x
				case *expvar.Float:
					x, _ := strconv.ParseFloat(v.String(), 64)
					fields[prefix+kv.Key] = x
				case *expvar.String:
					x, _ := strconv.Unquote(v.String())
					fields[prefix+kv.Key] = x
				case IntFunc:
					fields[prefix+kv.Key] = v.Int()
				case *expvar.Map:
					v.Do(func(x expvar.KeyValue) { do(kv.Key+".", x) })
				default:
					fields[prefix+kv.Key] = v.String()
				}
			}
			v.Do(func(kv expvar.KeyValue) { do("", kv) })

			if len(fields) == 0 {
				continue
			}

			bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
				Database:  "gitarchive42",
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
