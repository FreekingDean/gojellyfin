// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package store

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/versioned-migration,sql/upsert --template ./templates ./entities
//--go:generate atlas migrate diff schema --dir "file://migrations" --to "ent://entities" --dev-url "docker://postgres/16/dev?search_path=public"
