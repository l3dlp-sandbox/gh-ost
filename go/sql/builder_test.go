/*
   Copyright 2016 GitHub Inc.
	 See https://github.com/github/gh-osc/blob/master/LICENSE
*/

package sql

import (
	"testing"

	"reflect"
	"regexp"
	"strings"

	"github.com/outbrain/golib/log"
	test "github.com/outbrain/golib/tests"
)

var (
	spacesRegexp = regexp.MustCompile(`[ \t\n\r]+`)
)

func init() {
	log.SetLevel(log.ERROR)
}

func normalizeQuery(name string) string {
	name = strings.Replace(name, "`", "", -1)
	name = spacesRegexp.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)
	return name
}

func TestEscapeName(t *testing.T) {
	names := []string{"my_table", `"my_table"`, "`my_table`"}
	for _, name := range names {
		escaped := EscapeName(name)
		test.S(t).ExpectEquals(escaped, "`my_table`")
	}
}

func TestBuildEqualsComparison(t *testing.T) {
	{
		columns := []string{"c1"}
		values := []string{"@v1"}
		comparison, err := BuildEqualsComparison(columns, values)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` = @v1))")
	}
	{
		columns := []string{"c1", "c2"}
		values := []string{"@v1", "@v2"}
		comparison, err := BuildEqualsComparison(columns, values)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` = @v1) and (`c2` = @v2))")
	}
	{
		columns := []string{"c1"}
		values := []string{"@v1", "@v2"}
		_, err := BuildEqualsComparison(columns, values)
		test.S(t).ExpectNotNil(err)
	}
	{
		columns := []string{}
		values := []string{}
		_, err := BuildEqualsComparison(columns, values)
		test.S(t).ExpectNotNil(err)
	}
}

func TestBuildRangeComparison(t *testing.T) {
	{
		columns := []string{"c1"}
		values := []string{"@v1"}
		args := []interface{}{3}
		comparison, explodedArgs, err := BuildRangeComparison(columns, values, args, LessThanComparisonSign)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` < @v1))")
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3}))
	}
	{
		columns := []string{"c1"}
		values := []string{"@v1"}
		args := []interface{}{3}
		comparison, explodedArgs, err := BuildRangeComparison(columns, values, args, LessThanOrEqualsComparisonSign)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` < @v1) or ((`c1` = @v1)))")
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3}))
	}
	{
		columns := []string{"c1", "c2"}
		values := []string{"@v1", "@v2"}
		args := []interface{}{3, 17}
		comparison, explodedArgs, err := BuildRangeComparison(columns, values, args, LessThanComparisonSign)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` < @v1) or (((`c1` = @v1)) AND (`c2` < @v2)))")
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17}))
	}
	{
		columns := []string{"c1", "c2"}
		values := []string{"@v1", "@v2"}
		args := []interface{}{3, 17}
		comparison, explodedArgs, err := BuildRangeComparison(columns, values, args, LessThanOrEqualsComparisonSign)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` < @v1) or (((`c1` = @v1)) AND (`c2` < @v2)) or ((`c1` = @v1) and (`c2` = @v2)))")
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17, 3, 17}))
	}
	{
		columns := []string{"c1", "c2", "c3"}
		values := []string{"@v1", "@v2", "@v3"}
		args := []interface{}{3, 17, 22}
		comparison, explodedArgs, err := BuildRangeComparison(columns, values, args, LessThanOrEqualsComparisonSign)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(comparison, "((`c1` < @v1) or (((`c1` = @v1)) AND (`c2` < @v2)) or (((`c1` = @v1) and (`c2` = @v2)) AND (`c3` < @v3)) or ((`c1` = @v1) and (`c2` = @v2) and (`c3` = @v3)))")
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17, 3, 17, 22, 3, 17, 22}))
	}
	{
		columns := []string{"c1"}
		values := []string{"@v1", "@v2"}
		args := []interface{}{3, 17}
		_, _, err := BuildRangeComparison(columns, values, args, LessThanOrEqualsComparisonSign)
		test.S(t).ExpectNotNil(err)
	}
	{
		columns := []string{}
		values := []string{}
		args := []interface{}{}
		_, _, err := BuildRangeComparison(columns, values, args, LessThanOrEqualsComparisonSign)
		test.S(t).ExpectNotNil(err)
	}
}

func TestBuildRangeInsertQuery(t *testing.T) {
	databaseName := "mydb"
	originalTableName := "tbl"
	ghostTableName := "ghost"
	sharedColumns := []string{"id", "name", "position"}
	{
		uniqueKey := "PRIMARY"
		uniqueKeyColumns := []string{"id"}
		rangeStartValues := []string{"@v1s"}
		rangeEndValues := []string{"@v1e"}
		rangeStartArgs := []interface{}{3}
		rangeEndArgs := []interface{}{103}

		query, explodedArgs, err := BuildRangeInsertQuery(databaseName, originalTableName, ghostTableName, sharedColumns, uniqueKey, uniqueKeyColumns, rangeStartValues, rangeEndValues, rangeStartArgs, rangeEndArgs, true)
		test.S(t).ExpectNil(err)
		expected := `
				insert /* gh-osc mydb.tbl */ ignore into mydb.ghost (id, name, position)
				(select id, name, position from mydb.tbl force index (PRIMARY)
					where (((id > @v1s) or ((id = @v1s))) and ((id < @v1e) or ((id = @v1e))))
				)
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 103, 103}))
	}
	{
		uniqueKey := "name_position_uidx"
		uniqueKeyColumns := []string{"name", "position"}
		rangeStartValues := []string{"@v1s", "@v2s"}
		rangeEndValues := []string{"@v1e", "@v2e"}
		rangeStartArgs := []interface{}{3, 17}
		rangeEndArgs := []interface{}{103, 117}

		query, explodedArgs, err := BuildRangeInsertQuery(databaseName, originalTableName, ghostTableName, sharedColumns, uniqueKey, uniqueKeyColumns, rangeStartValues, rangeEndValues, rangeStartArgs, rangeEndArgs, true)
		test.S(t).ExpectNil(err)
		expected := `
				insert /* gh-osc mydb.tbl */ ignore into mydb.ghost (id, name, position)
				(select id, name, position from mydb.tbl force index (name_position_uidx)
				  where (((name > @v1s) or (((name = @v1s)) AND (position > @v2s)) or ((name = @v1s) and (position = @v2s))) and ((name < @v1e) or (((name = @v1e)) AND (position < @v2e)) or ((name = @v1e) and (position = @v2e))))
				)
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17, 3, 17, 103, 103, 117, 103, 117}))
	}
}

func TestBuildRangeInsertPreparedQuery(t *testing.T) {
	databaseName := "mydb"
	originalTableName := "tbl"
	ghostTableName := "ghost"
	sharedColumns := []string{"id", "name", "position"}
	{
		uniqueKey := "name_position_uidx"
		uniqueKeyColumns := []string{"name", "position"}
		rangeStartArgs := []interface{}{3, 17}
		rangeEndArgs := []interface{}{103, 117}

		query, explodedArgs, err := BuildRangeInsertPreparedQuery(databaseName, originalTableName, ghostTableName, sharedColumns, uniqueKey, uniqueKeyColumns, rangeStartArgs, rangeEndArgs, true)
		test.S(t).ExpectNil(err)
		expected := `
				insert /* gh-osc mydb.tbl */ ignore into mydb.ghost (id, name, position)
				(select id, name, position from mydb.tbl force index (name_position_uidx)
				  where (((name > ?) or (((name = ?)) AND (position > ?)) or ((name = ?) and (position = ?))) and ((name < ?) or (((name = ?)) AND (position < ?)) or ((name = ?) and (position = ?))))
				)
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17, 3, 17, 103, 103, 117, 103, 117}))
	}
}

func TestBuildUniqueKeyRangeEndPreparedQuery(t *testing.T) {
	databaseName := "mydb"
	originalTableName := "tbl"
	chunkSize := 500
	{
		uniqueKeyColumns := []string{"name", "position"}
		rangeStartArgs := []interface{}{3, 17}
		rangeEndArgs := []interface{}{103, 117}

		query, explodedArgs, err := BuildUniqueKeyRangeEndPreparedQuery(databaseName, originalTableName, uniqueKeyColumns, rangeStartArgs, rangeEndArgs, chunkSize, "test")
		test.S(t).ExpectNil(err)
		expected := `
				select /* gh-osc mydb.tbl test */ name, position
				  from (
				    select
				        name, position
				      from
				        mydb.tbl
				      where ((name > ?) or (((name = ?)) AND (position > ?))) and ((name < ?) or (((name = ?)) AND (position < ?)) or ((name = ?) and (position = ?)))
				      order by
				        name asc, position asc
				      limit 500
				  ) select_osc_chunk
				order by
				  name desc, position desc
				limit 1
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
		test.S(t).ExpectTrue(reflect.DeepEqual(explodedArgs, []interface{}{3, 3, 17, 103, 103, 117, 103, 117}))
	}
}

func TestBuildUniqueKeyMinValuesPreparedQuery(t *testing.T) {
	databaseName := "mydb"
	originalTableName := "tbl"
	uniqueKeyColumns := []string{"name", "position"}
	{
		query, err := BuildUniqueKeyMinValuesPreparedQuery(databaseName, originalTableName, uniqueKeyColumns)
		test.S(t).ExpectNil(err)
		expected := `
			select /* gh-osc mydb.tbl */ name, position
			  from
			    mydb.tbl
			  order by
			    name asc, position asc
			  limit 1
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
	}
	{
		query, err := BuildUniqueKeyMaxValuesPreparedQuery(databaseName, originalTableName, uniqueKeyColumns)
		test.S(t).ExpectNil(err)
		expected := `
			select /* gh-osc mydb.tbl */ name, position
			  from
			    mydb.tbl
			  order by
			    name desc, position desc
			  limit 1
		`
		test.S(t).ExpectEquals(normalizeQuery(query), normalizeQuery(expected))
	}
}