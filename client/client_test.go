// Copyright 2024 Greptime Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/GreptimeTeam/greptimedb-ingester-go/config"
	tbl "github.com/GreptimeTeam/greptimedb-ingester-go/table"
	"github.com/GreptimeTeam/greptimedb-ingester-go/table/types"
)

var (
	tableName = ""
	timezone  = "UTC"

	database                      = "public"
	host                          = "127.0.0.1"
	httpPort, grpcPort, mysqlPort = 4000, 4001, 4002

	cli *Client
	db  *Mysql
)

// this is to scan all datatypes from GreptimeDB
type datatype struct {
	INT8    int8    `gorm:"primaryKey;column:int8"`
	INT16   int16   `gorm:"column:int16"`
	INT32   int32   `gorm:"column:int32"`
	INT64   int64   `gorm:"column:int64"`
	UINT8   uint8   `gorm:"column:uint8"`
	UINT16  uint16  `gorm:"column:uint16"`
	UINT32  uint32  `gorm:"column:uint32"`
	UINT64  uint64  `gorm:"column:uint64"`
	BOOLEAN bool    `gorm:"column:boolean"`
	FLOAT32 float32 `gorm:"column:float32"`
	FLOAT64 float64 `gorm:"column:float64"`
	BINARY  []byte  `gorm:"column:binary"`
	STRING  string  `gorm:"column:string"`

	DATE                  time.Time `gorm:"column:date"`
	DATETIME              time.Time `gorm:"column:datetime"`
	TIMESTAMP_SECOND      time.Time `gorm:"column:timestamp_second"`
	TIMESTAMP_MILLISECOND time.Time `gorm:"column:timestamp_millisecond"`
	TIMESTAMP_MICROSECOND time.Time `gorm:"column:timestamp_microsecond"`
	TIMESTAMP_NANOSECOND  time.Time `gorm:"column:timestamp_nanosecond"`

	DATE_INT                  time.Time `gorm:"column:date_int"`
	DATETIME_INT              time.Time `gorm:"column:datetime_int"`
	TIMESTAMP_SECOND_INT      time.Time `gorm:"column:timestamp_second_int"`
	TIMESTAMP_MILLISECOND_INT time.Time `gorm:"column:timestamp_millisecond_int"`
	TIMESTAMP_MICROSECOND_INT time.Time `gorm:"column:timestamp_microsecond_int"`
	TIMESTAMP_NANOSECOND_INT  time.Time `gorm:"column:timestamp_nanosecond_int"`

	TS time.Time `gorm:"column:ts"`
}

func (datatype) TableName() string {
	return tableName
}

type monitor struct {
	ID          int64     `gorm:"primaryKey;column:id"`
	Host        string    `gorm:"primaryKey;column:host"`
	Memory      uint64    `gorm:"column:memory"`
	Cpu         float64   `gorm:"column:cpu"`
	Temperature int64     `gorm:"column:temperature"`
	Running     bool      `gorm:"column:running"`
	Ts          time.Time `gorm:"column:ts"`
}

func (monitor) TableName() string {
	return tableName
}

type Mysql struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string

	DB *gorm.DB
}

func (m *Mysql) Setup() error {
	if m.DB != nil {
		return nil
	}

	dsn := fmt.Sprintf("tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=%s",
		m.Host, m.Port, m.Database, timezone)
	if m.User != "" && m.Password != "" {
		dsn = fmt.Sprintf("%s:%s@%s", m.User, m.Password, dsn)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	m.DB = db
	return nil
}

func (p *Mysql) Query(sql string) ([]monitor, error) {
	var monitors []monitor
	err := p.DB.Raw(sql).Scan(&monitors).Error
	return monitors, err
}

func (p *Mysql) AllDatatypes() ([]datatype, error) {
	var datatypes []datatype
	err := p.DB.Find(&datatypes).Error
	return datatypes, err
}

func newClient() *Client {
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	cfg := config.New(host).
		WithPort(grpcPort).
		WithDatabase(database).
		WithDialOptions(options...)

	client, err := New(cfg)
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}
	return client
}

func newMysql() *Mysql {
	db := &Mysql{
		Host:     host,
		Port:     mysqlPort,
		User:     "",
		Password: "",
		Database: database,
	}
	if err := db.Setup(); err != nil {
		log.Fatalln("failed to setup mysql" + err.Error())
	}
	return db
}

func init() {
	repo := "greptime/greptimedb"
	tag := "v0.6.0"

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalln("Could not connect to docker: " + err.Error())
	}

	log.Printf("Preparing container %s:%s\n", repo, tag)

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   repo,
		Tag:          tag,
		ExposedPorts: []string{"4000", "4001", "4002"},
		Entrypoint: []string{"greptime", "standalone", "start",
			"--http-addr=0.0.0.0:4000",
			"--rpc-addr=0.0.0.0:4001",
			"--mysql-addr=0.0.0.0:4002"},
	}, func(config *dc.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = dc.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalln("could not start resource: " + err.Error())
	}
	var expire uint = 30
	log.Println("Starting container...")

	err = resource.Expire(expire) // Tell docker to hard kill the container
	if err != nil {
		log.Printf("Expire container failed, %s\n", err.Error())
	}

	pool.MaxWait = 30 * time.Second

	if err := pool.Retry(func() error {
		time.Sleep(time.Second * 5)
		httpPort, err = strconv.Atoi(resource.GetPort(("4000/tcp")))
		grpcPort, err = strconv.Atoi(resource.GetPort(("4001/tcp")))
		mysqlPort, err = strconv.Atoi(resource.GetPort(("4002/tcp")))
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	log.Printf("Container started, http port: %d, grpc port: %d, mysql port: %d\n", httpPort, grpcPort, mysqlPort)

	cli = newClient()
	db = newMysql()
}

func TestInsertMonitors(t *testing.T) {
	tableName = "test_insert_monitor"

	loc, err := time.LoadLocation(timezone)
	assert.Nil(t, err)
	ts1 := time.Now().Add(-1 * time.Minute).UnixMilli()
	time1 := time.UnixMilli(ts1).In(loc)
	ts2 := time.Now().Add(-2 * time.Minute).UnixMilli()
	time2 := time.UnixMilli(ts2).In(loc)

	monitors := []monitor{
		{
			ID:          1,
			Host:        "127.0.0.1",
			Memory:      1,
			Cpu:         1.0,
			Temperature: -1,
			Ts:          time1,
			Running:     true,
		},
		{
			ID:          2,
			Host:        "127.0.0.2",
			Memory:      2,
			Cpu:         2.0,
			Temperature: -2,
			Ts:          time2,
			Running:     true,
		},
	}

	table, err := tbl.New(tableName)
	assert.Nil(t, err)

	assert.Nil(t, table.AddTagColumn("id", types.INT64))
	assert.Nil(t, table.AddTagColumn("host", types.STRING))
	assert.Nil(t, table.AddFieldColumn("memory", types.UINT64))
	assert.Nil(t, table.AddFieldColumn("cpu", types.FLOAT64))
	assert.Nil(t, table.AddFieldColumn("temperature", types.INT64))
	assert.Nil(t, table.AddFieldColumn("running", types.BOOLEAN))
	assert.Nil(t, table.AddTimestampColumn("ts", types.TIMESTAMP_MILLISECOND))

	for _, monitor := range monitors {
		err := table.AddRow(monitor.ID, monitor.Host,
			monitor.Memory, monitor.Cpu, monitor.Temperature, monitor.Running,
			monitor.Ts)
		assert.Nil(t, err)
	}

	resp, err := cli.Write(context.Background(), table)
	assert.Nil(t, err)
	assert.Zero(t, resp.GetHeader().GetStatus().GetStatusCode())
	assert.Empty(t, resp.GetHeader().GetStatus().GetErrMsg())
	assert.Equal(t, uint32(len(monitors)), resp.GetAffectedRows().GetValue())

	monitors_, err := db.Query(fmt.Sprintf("select * from %s where id in (1, 2) order by id asc", tableName))
	assert.Nil(t, err)

	assert.Equal(t, len(monitors), len(monitors_))

	for i, monitor_ := range monitors_ {
		assert.Equal(t, monitors[i], monitor_)
	}
}

func TestInsertMonitorWithNilFields(t *testing.T) {
	tableName = "test_insert_monitor_with_nil_fields"

	loc, err := time.LoadLocation(timezone)
	assert.Nil(t, err)
	ts := time.Now().Add(-1 * time.Minute).UnixMilli()
	time := time.UnixMilli(ts).In(loc)
	monitor := monitor{
		ID:          11,
		Host:        "127.0.0.1",
		Memory:      1,
		Cpu:         1.0,
		Temperature: -1,
		Ts:          time,
		Running:     true,
	}

	table, err := tbl.New(tableName)
	assert.Nil(t, err)

	assert.Nil(t, table.AddTagColumn("id", types.INT64))
	assert.Nil(t, table.AddTagColumn("host", types.STRING))
	assert.Nil(t, table.AddFieldColumn("memory", types.UINT64))
	assert.Nil(t, table.AddFieldColumn("cpu", types.FLOAT64))
	assert.Nil(t, table.AddFieldColumn("temperature", types.INT64))
	assert.Nil(t, table.AddFieldColumn("running", types.BOOLEAN))
	assert.Nil(t, table.AddTimestampColumn("ts", types.TIMESTAMP_MILLISECOND))

	// with nil fields
	err = table.AddRow(monitor.ID, monitor.Host, nil, nil, nil, monitor.Running, monitor.Ts)
	assert.Nil(t, err)

	resp, err := cli.Write(context.Background(), table)
	assert.Nil(t, err)
	assert.Zero(t, resp.GetHeader().GetStatus().GetStatusCode())
	assert.Empty(t, resp.GetHeader().GetStatus().GetErrMsg())

	monitors_, err := db.Query(fmt.Sprintf("select * from %s where id = %d", tableName, monitor.ID))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(monitors_))
	monitor_ := monitors_[0]

	assert.Equal(t, monitor.ID, monitor_.ID)
	assert.Equal(t, monitor.Host, monitor_.Host)
	assert.Equal(t, monitor.Running, monitor_.Running)
	assert.Equal(t, monitor.Ts, monitor_.Ts)

	assert.Zero(t, monitor_.Memory)
	assert.Zero(t, monitor_.Cpu)
	assert.Zero(t, monitor_.Temperature)
}

func TestInsertMonitorWithAllDatatypes(t *testing.T) {
	tableName = "test_insert_monitor_with_all_datatypes"

	loc, err := time.LoadLocation(timezone)
	assert.Nil(t, err)

	time_ := time.Now().In(loc)
	date_int := time_.Unix() / 86400
	datetime_int := time_.UnixMilli()

	INT8 := 1
	INT16 := 2
	INT32 := 3
	INT64 := 4
	UINT8 := 5
	UINT16 := 6
	UINT32 := 7
	UINT64 := 8
	BOOLEAN := true
	FLOAT32 := 9.0
	FLOAT64 := 10.0
	BINARY := []byte{1, 2, 3}
	STRING := "string"

	table, err := tbl.New(tableName)
	assert.Nil(t, err)

	assert.Nil(t, table.AddTagColumn("int8", types.INT8))
	assert.Nil(t, table.AddFieldColumn("int16", types.INT16))
	assert.Nil(t, table.AddFieldColumn("int32", types.INT32))
	assert.Nil(t, table.AddFieldColumn("int64", types.INT64))
	assert.Nil(t, table.AddFieldColumn("uint8", types.UINT8))
	assert.Nil(t, table.AddFieldColumn("uint16", types.UINT16))
	assert.Nil(t, table.AddFieldColumn("uint32", types.UINT32))
	assert.Nil(t, table.AddFieldColumn("uint64", types.UINT64))
	assert.Nil(t, table.AddFieldColumn("boolean", types.BOOLEAN))
	assert.Nil(t, table.AddFieldColumn("float32", types.FLOAT32))
	assert.Nil(t, table.AddFieldColumn("float64", types.FLOAT64))
	assert.Nil(t, table.AddFieldColumn("binary", types.BINARY))
	assert.Nil(t, table.AddFieldColumn("string", types.STRING))

	assert.Nil(t, table.AddFieldColumn("date", types.DATE))
	assert.Nil(t, table.AddFieldColumn("datetime", types.DATETIME))
	assert.Nil(t, table.AddFieldColumn("timestamp_second", types.TIMESTAMP_SECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_millisecond", types.TIMESTAMP_MILLISECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_microsecond", types.TIMESTAMP_MICROSECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_nanosecond", types.TIMESTAMP_NANOSECOND))

	assert.Nil(t, table.AddFieldColumn("date_int", types.DATE))
	assert.Nil(t, table.AddFieldColumn("datetime_int", types.DATETIME))
	assert.Nil(t, table.AddFieldColumn("timestamp_second_int", types.TIMESTAMP_SECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_millisecond_int", types.TIMESTAMP_MILLISECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_microsecond_int", types.TIMESTAMP_MICROSECOND))
	assert.Nil(t, table.AddFieldColumn("timestamp_nanosecond_int", types.TIMESTAMP_NANOSECOND))

	assert.Nil(t, table.AddTimestampColumn("ts", types.TIMESTAMP_MILLISECOND))

	// with all fields
	err = table.AddRow(INT8, INT16, INT32, INT64,
		UINT8, UINT16, UINT32, UINT64,
		BOOLEAN, FLOAT32, FLOAT64,
		BINARY, STRING,

		time_, time_, // date and datetime
		time_, time_, time_, time_, // timestamp

		date_int, datetime_int, // date and datetime
		time_.Unix(), time_.UnixMilli(), time_.UnixMicro(), time_.UnixNano(), // timestamp

		time_)
	assert.Nil(t, err)

	resp, err := cli.Write(context.Background(), table)
	assert.Nil(t, err)
	assert.Zero(t, resp.GetHeader().GetStatus().GetStatusCode())
	assert.Empty(t, resp.GetHeader().GetStatus().GetErrMsg())

	datatypes, err := db.AllDatatypes()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(datatypes))
	result := datatypes[0]

	assert.EqualValues(t, INT8, result.INT8)
	assert.EqualValues(t, INT16, result.INT16)
	assert.EqualValues(t, INT32, result.INT32)
	assert.EqualValues(t, INT64, result.INT64)
	assert.EqualValues(t, UINT8, result.UINT8)
	assert.EqualValues(t, UINT16, result.UINT16)
	assert.EqualValues(t, UINT32, result.UINT32)
	assert.EqualValues(t, UINT64, result.UINT64)
	assert.EqualValues(t, BOOLEAN, result.BOOLEAN)
	assert.EqualValues(t, FLOAT32, result.FLOAT32)
	assert.EqualValues(t, FLOAT64, result.FLOAT64)
	assert.EqualValues(t, BINARY, result.BINARY)
	assert.EqualValues(t, STRING, result.STRING)

	assert.Equal(t, time_.Format("2006-01-02"), result.DATE.Format("2006-01-02"))
	assert.Equal(t, time_.Format("2006-01-02 15:04:05"), result.DATETIME.Format("2006-01-02 15:04:05"))
	assert.Equal(t, time_.Unix(), result.TIMESTAMP_SECOND.Unix())
	assert.Equal(t, time_.UnixMilli(), result.TIMESTAMP_MILLISECOND.UnixMilli())
	assert.Equal(t, time_.UnixMicro(), result.TIMESTAMP_MICROSECOND.UnixMicro())

	// MySQL protocol only supports microsecond precision for TIMESTAMP
	assert.EqualValues(t, time_.UnixNano()/1000, result.TIMESTAMP_NANOSECOND.UnixNano()/1000)

	assert.Equal(t, time_.Format("2006-01-02"), result.DATE_INT.Format("2006-01-02"))
	assert.Equal(t, time_.Format("2006-01-02 15:04:05"), result.DATETIME_INT.Format("2006-01-02 15:04:05"))
	assert.Equal(t, time_.Unix(), result.TIMESTAMP_SECOND_INT.Unix())
	assert.Equal(t, time_.UnixMilli(), result.TIMESTAMP_MILLISECOND_INT.UnixMilli())
	assert.Equal(t, time_.UnixMicro(), result.TIMESTAMP_MICROSECOND_INT.UnixMicro())

	// MySQL protocol only supports microsecond precision for TIMESTAMP
	assert.EqualValues(t, time_.UnixNano()/1000, result.TIMESTAMP_NANOSECOND_INT.UnixNano()/1000)
}

//TODO(yuanbohan):
// unmatched length of columns in rows and columns in schema
// support pointer
// write pojo
