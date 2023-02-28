package request

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	greptime "github.com/GreptimeTeam/greptime-proto/go/greptime/v1"
)

type column struct {
	typ      greptime.ColumnDataType
	semantic greptime.Column_SemanticType
}

type Series struct {
	// order, columns and vals SHOULD NOT contain timestampAlias
	order   []string
	columns map[string]column
	vals    map[string]any

	timestampAlias string
	timestamp      time.Time
}

func checkColumnEquality(key string, col1, col2 column) error {
	if col1.typ != col2.typ {
		return fmt.Errorf("the type of '%s' does not match: '%v' and '%v'", key, col1.typ, col2.typ)
	}
	if col1.semantic != col2.semantic {
		return fmt.Errorf("tag and field MUST NOT contain same name")
	}

	return nil
}

func (s *Series) addVal(key string, val any, semantic greptime.Column_SemanticType) error {
	if IsEmptyString(key) {
		return ErrEmptyKey
	}
	key = ToColumnName(key)

	if s.columns == nil {
		s.columns = map[string]column{}
	}

	v, err := convert(val)
	if err != nil {
		return fmt.Errorf("add tag err: %w", err)
	}

	col, seen := s.columns[key]
	newCol := column{
		typ:      v.typ,
		semantic: semantic,
	}
	if seen {
		if err := checkColumnEquality(key, col, newCol); err != nil {
			return err
		}
	}
	s.columns[key] = newCol
	s.order = append(s.order, key)

	if s.vals == nil {
		s.vals = map[string]any{}
	}
	s.vals[key] = v.val

	return nil
}

// AddTag prepate tag column, and old value will be replaced if same tag is set
func (s *Series) AddTag(key string, val any) error {
	return s.addVal(key, val, greptime.Column_TAG)
}

// AddField prepate field column, and old value will be replaced if same field is set
func (s *Series) AddField(key string, val any) error {
	return s.addVal(key, val, greptime.Column_FIELD)
}

func (s *Series) SetTime(t time.Time) error {
	return s.SetTimeWithKey("ts", t)
}

func (s *Series) SetTimeWithKey(key string, t time.Time) error {
	if len(s.timestampAlias) != 0 {
		return errors.New("timestamp column name CAN NOT be set twice")
	}

	if IsEmptyString(key) {
		return ErrEmptyKey
	}

	s.timestampAlias = ToColumnName(key)
	s.timestamp = t
	return nil
}

type Metric struct {
	timestampAlias string
	// order and columns SHOULD NOT contain timestampAlias key
	order   []string
	columns map[string]column

	series []Series
}

func (m *Metric) AddSeries(s Series) error {
	if !IsEmptyString(m.timestampAlias) && !IsEmptyString(s.timestampAlias) &&
		!strings.EqualFold(m.timestampAlias, s.timestampAlias) {
		return fmt.Errorf("different series MUST share same timestamp key, '%s' and '%s' does not match",
			m.timestampAlias, s.timestampAlias)
	} else if IsEmptyString(m.timestampAlias) && !IsEmptyString(s.timestampAlias) {
		m.timestampAlias = s.timestampAlias
	}

	if m.columns == nil {
		m.columns = map[string]column{}
	}
	for _, key := range s.order {
		sCol := s.columns[key]
		mCol, seen := m.columns[key]
		if seen {
			if err := checkColumnEquality(key, mCol, sCol); err != nil {
				return err
			}
		} else {
			m.order = append(m.order, key)
			m.columns[key] = sCol
		}
	}

	m.series = append(m.series, s)
	return nil
}

func (m *Metric) IntoGreptimeColumn() ([]*greptime.Column, error) {
	if len(m.series) == 0 {
		return nil, errors.New("empty series in Metric")
	}

	result, err := m.normalColumns()
	if err != nil {
		return nil, err
	}

	tsColumn, err := m.timestampColumn()
	if err != nil {
		return nil, err
	}

	return append(result, tsColumn), nil
}

func (m *Metric) nullMaskByteSize() int {
	return int(math.Ceil(float64(len(m.series)) / 8.0))
}

// normalColumns does not contain timestamp semantic column
func (m *Metric) normalColumns() ([]*greptime.Column, error) {
	nullMasks := map[string]*Mask{}
	mappedCols := map[string]*greptime.Column{}
	for name, col := range m.columns {
		column := greptime.Column{
			ColumnName:   name,
			SemanticType: col.semantic,
			Datatype:     col.typ,
			Values:       &greptime.Column_Values{},
			NullMask:     nil,
		}
		mappedCols[name] = &column
	}

	for rowIdx, s := range m.series {
		for name, col := range mappedCols {
			if val, exist := s.vals[name]; exist {
				if err := setColumn(col, val); err != nil {
					return nil, err
				}
			} else {
				mask, exist := nullMasks[name]
				if !exist {
					mask = &Mask{}
					nullMasks[name] = mask
				}
				mask.set(uint(rowIdx))
			}
		}
	}

	if len(nullMasks) > 0 {
		if err := setNullMask(mappedCols, nullMasks, m.nullMaskByteSize()); err != nil {
			return nil, err
		}
	}

	result := make([]*greptime.Column, 0, len(mappedCols))
	for _, key := range m.order {
		result = append(result, mappedCols[key])
	}

	return result, nil
}

func (m *Metric) timestampColumn() (*greptime.Column, error) {
	tsColumn := &greptime.Column{
		ColumnName:   m.timestampAlias,
		SemanticType: greptime.Column_TIMESTAMP,
		Datatype:     greptime.ColumnDataType_TIMESTAMP_MILLISECOND,
		Values:       &greptime.Column_Values{},
		NullMask:     nil,
	}
	nullMask := Mask{}
	for rowIdx, s := range m.series {
		if !IsEmptyString(s.timestampAlias) {
			setColumn(tsColumn, s.timestamp.UnixMilli())
		} else {
			nullMask.set(uint(rowIdx))
		}
	}

	if b, err := nullMask.shrink(m.nullMaskByteSize()); err != nil {
		return nil, err
	} else {
		tsColumn.NullMask = b
	}

	return tsColumn, nil
}

func setColumn(col *greptime.Column, val any) error {
	switch col.Datatype {
	case greptime.ColumnDataType_BOOLEAN:
		col.Values.BoolValues = append(col.Values.BoolValues, val.(bool))
	case greptime.ColumnDataType_INT32:
		col.Values.I32Values = append(col.Values.I32Values, val.(int32))
	case greptime.ColumnDataType_INT64:
		col.Values.I64Values = append(col.Values.I64Values, val.(int64))
	case greptime.ColumnDataType_UINT32:
		col.Values.U32Values = append(col.Values.U32Values, val.(uint32))
	case greptime.ColumnDataType_UINT64:
		col.Values.U64Values = append(col.Values.U64Values, val.(uint64))
	case greptime.ColumnDataType_FLOAT32:
		col.Values.F32Values = append(col.Values.F32Values, val.(float32))
	case greptime.ColumnDataType_FLOAT64:
		col.Values.F64Values = append(col.Values.F64Values, val.(float64))
	case greptime.ColumnDataType_STRING:
		col.Values.StringValues = append(col.Values.StringValues, val.(string))
	case greptime.ColumnDataType_TIMESTAMP_MILLISECOND:
		col.Values.TsMillisecondValues = append(col.Values.TsMillisecondValues, val.(int64))
	default:
		return fmt.Errorf("unknown column data type: %v", col.Datatype)
	}
	return nil
}

func setNullMask(cols map[string]*greptime.Column, masks map[string]*Mask, size int) error {
	for name, mask := range masks {
		b, err := mask.shrink(size)
		if err != nil {
			return err
		}

		col, exist := cols[name]
		if !exist {
			return fmt.Errorf("%v column not found when set null mask", name)
		}
		col.NullMask = b
	}

	return nil
}