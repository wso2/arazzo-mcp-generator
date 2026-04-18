/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package metadata holds static CLI metadata such as the version string.
// When releasing a new version, update the Version constant below.
package metadata

// Version is the current version of arazzo-mcp-gen.
// It defaults to "v1.0.0" locally and is overridden at build time via
// -ldflags "-X github.com/wso2/arazzo-mcp-generator/internal/metadata.Version=<tag>".
var Version = "v1.0.0"
