// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statecache

import (
	"context"
	"testing"

	"github.com/apache/beam/sdks/v2/go/pkg/beam/core/runtime/exec"
	fnpb "github.com/apache/beam/sdks/v2/go/pkg/beam/model/fnexecution_v1"
)

func makeTestReStream(value interface{}) exec.ReStream {
	fv := exec.FullValue{Elm: value}
	return &exec.FixedReStream{Buf: []exec.FullValue{fv}}
}

func getValue(rs exec.ReStream) interface{} {
	stream, _ := rs.Open()
	fullVal, _ := stream.Read()
	return fullVal.Elm
}

func TestInit(t *testing.T) {
	var s SideInputCache
	err := s.Init(5)
	if err != nil {
		t.Errorf("SideInputCache failed but should have succeeded, got %v", err)
	}
}

func TestInit_Bad(t *testing.T) {
	var s SideInputCache
	err := s.Init(0)
	if err == nil {
		t.Error("SideInputCache init succeeded but should have failed")
	}
}

func TestQueryCache_EmptyCase(t *testing.T) {
	var s SideInputCache
	ctx := context.Background()
	win := []byte{0}
	key := []byte{1}
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}
	output := s.QueryCache(ctx, "side1", "transform1", win, key)
	if output != nil {
		t.Errorf("Cache hit when it should have missed, got %v", output)
	}
}

func TestSetCache_UncacheableCase(t *testing.T) {
	var s SideInputCache
	ctx := context.Background()
	win := []byte{0}
	key := []byte{1}
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}
	input := makeTestReStream(10)
	s.SetCache(ctx, "t1", "s1", win, key, input)
	output := s.QueryCache(ctx, "t1", "s1", win, key)
	if output != nil {
		t.Errorf("Cache hit when should have missed, got %v", output)
	}
}

func TestSetCache_CacheableCase(t *testing.T) {
	var s SideInputCache
	ctx := context.Background()
	win := []byte{0}
	key := []byte{1}
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}
	transID := "t1"
	sideID := "s1"
	tok := token("tok1")
	s.setValidToken(transID, sideID, tok)
	input := makeTestReStream(10)
	s.SetCache(ctx, transID, sideID, win, key, input)
	output := s.QueryCache(ctx, transID, sideID, win, key)
	if output == nil {
		t.Fatalf("call to query cache missed when should have hit")
	}
	val, ok := getValue(output).(int)
	if !ok {
		t.Errorf("failed to convert value to integer, got %v", getValue(output))
	}
	if val != 10 {
		t.Errorf("element mismatch, expected 10, got %v", val)
	}
}

func makeRequest(transformID, sideInputID string, t token) *fnpb.ProcessBundleRequest_CacheToken {
	return &fnpb.ProcessBundleRequest_CacheToken{
		Token: []byte(t),
		Type: &fnpb.ProcessBundleRequest_CacheToken_SideInput_{
			SideInput: &fnpb.ProcessBundleRequest_CacheToken_SideInput{
				TransformId: transformID,
				SideInputId: sideInputID,
			},
		},
	}
}

func TestSetValidTokens(t *testing.T) {
	inputs := []struct {
		transformID string
		sideInputID string
		tok         token
	}{
		{
			"t1",
			"s1",
			"tok1",
		},
		{
			"t2",
			"s2",
			"tok2",
		},
		{
			"t3",
			"s3",
			"tok3",
		},
	}

	var s SideInputCache
	err := s.Init(3)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}

	var tokens []*fnpb.ProcessBundleRequest_CacheToken
	for _, input := range inputs {
		t := makeRequest(input.transformID, input.sideInputID, input.tok)
		tokens = append(tokens, t)
	}

	s.SetValidTokens(tokens...)
	if len(s.idsToTokens) != len(inputs) {
		t.Errorf("Missing tokens, expected %v, got %v", len(inputs), len(s.idsToTokens))
	}

	for i, input := range inputs {
		// Check that the token is in the valid list
		if !s.isValid(input.tok) {
			t.Errorf("error in input %v, token %v is not valid", i, input.tok)
		}
		// Check that the mapping of IDs to tokens is correct
		mapped := s.idsToTokens[input.transformID+input.sideInputID]
		if mapped != input.tok {
			t.Errorf("token mismatch for input %v, expected %v, got %v", i, input.tok, mapped)
		}
	}
}

func TestSetValidTokens_ClearingBetween(t *testing.T) {
	inputs := []struct {
		transformID string
		sideInputID string
		tk          token
	}{
		{
			"t1",
			"s1",
			"tok1",
		},
		{
			"t2",
			"s2",
			"tok2",
		},
		{
			"t3",
			"s3",
			"tok3",
		},
	}

	var s SideInputCache
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}

	for i, input := range inputs {
		tok := makeRequest(input.transformID, input.sideInputID, input.tk)

		s.SetValidTokens(tok)

		// Check that the token is in the valid list
		if !s.isValid(input.tk) {
			t.Errorf("error in input %v, token %v is not valid", i, input.tk)
		}
		// Check that the mapping of IDs to tokens is correct
		mapped := s.idsToTokens[input.transformID+input.sideInputID]
		if mapped != input.tk {
			t.Errorf("token mismatch for input %v, expected %v, got %v", i, input.tk, mapped)
		}

		s.CompleteBundle(tok)
	}

	for k := range s.validTokens {
		if s.validTokens[k] != 0 {
			t.Errorf("token count mismatch for token %v, expected 0, got %v", k, s.validTokens[k])
		}
	}
}

func TestSetCache_Eviction(t *testing.T) {
	var s SideInputCache
	ctx := context.Background()
	win := []byte{0}
	key := []byte{1}
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}

	tokOne := makeRequest("t1", "s1", "tok1")
	inOne := makeTestReStream(10)
	s.SetValidTokens(tokOne)
	s.SetCache(ctx, "t1", "s1", win, key, inOne)
	// Mark bundle as complete, drop count for tokOne to 0
	s.CompleteBundle(tokOne)

	tokTwo := makeRequest("t2", "s2", "tok2")
	inTwo := makeTestReStream(20)
	s.SetValidTokens(tokTwo)
	s.SetCache(ctx, "t2", "s2", win, key, inTwo)

	if len(s.cache) != 1 {
		t.Errorf("cache size incorrect, expected 1, got %v", len(s.cache))
	}
	if s.metrics.Evictions != 1 {
		t.Errorf("number evictions incorrect, expected 1, got %v", s.metrics.Evictions)
	}
}

func TestSetCache_EvictionFailure(t *testing.T) {
	var s SideInputCache
	ctx := context.Background()
	win := []byte{0}
	key := []byte{1}
	err := s.Init(1)
	if err != nil {
		t.Fatalf("cache init failed, got %v", err)
	}

	tokOne := makeRequest("t1", "s1", "tok1")
	inOne := makeTestReStream(10)

	tokTwo := makeRequest("t2", "s2", "tok2")
	inTwo := makeTestReStream(20)

	s.SetValidTokens(tokOne, tokTwo)
	s.SetCache(ctx, "t1", "s1", win, key, inOne)
	// Should fail to evict because the first token is still valid
	s.SetCache(ctx, "t2", "s2", win, key, inTwo)
	// Cache should not exceed size 1
	if len(s.cache) != 1 {
		t.Errorf("cache size incorrect, expected 1, got %v", len(s.cache))
	}
	if s.metrics.InUseEvictions != 1 {
		t.Errorf("number of failed evicition calls incorrect, expected 1, got %v", s.metrics.InUseEvictions)
	}
}
