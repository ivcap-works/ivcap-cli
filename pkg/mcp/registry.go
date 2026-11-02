// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// BuiltInTool describes a tool that the server always offers. The server derives everything else
// from these descriptors: the tools/list entries, their order, the session filter, the server
// instructions and the search index.
type BuiltInTool struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
	// ReadOnly is true if the tool only reads: it never changes data on the platform.
	ReadOnly bool
	// Instruction is an optional line of server instructions that tells the model when to use the tool.
	Instruction string
}

// registry holds the built-in tools by name. Tools in this package register themselves from init();
// cmd/ registers the tools that it implements (the genome tools) before the server is built.
var registry = map[string]BuiltInTool{}

// Register adds a built-in tool. Registering the same name twice is a programming error.
func Register(t BuiltInTool) {
	name := t.Tool.Name
	if _, dup := registry[name]; dup {
		panic("mcp: tool already registered: " + name)
	}
	registry[name] = t
}

// builtInToolNamesList returns the names of the registered tools in display order, without select_tools
// and search, which are described on their own in the server instructions.
func builtInToolNamesList() []string {
	var names []string
	for _, t := range builtInTools() {
		if n := t.Tool.Name; n != "select_tools" && n != "search" {
			names = append(names, n)
		}
	}
	return names
}

// isBuiltIn reports whether a tool name is a registered built-in tool.
func isBuiltIn(name string) bool {
	_, ok := registry[name]
	return ok
}

// builtInTools returns the registered tools in display order: select_tools first, then by name.
func builtInTools() []BuiltInTool {
	tools := make([]BuiltInTool, 0, len(registry))
	for _, t := range registry {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		a, b := tools[i].Tool.Name, tools[j].Tool.Name
		if (a == "select_tools") != (b == "select_tools") {
			return a == "select_tools"
		}
		return a < b
	})
	return tools
}
