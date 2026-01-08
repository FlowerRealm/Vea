package domain

// ClearNodes is a nil-slice sentinel.
//
// It is used by ReplaceNodesForConfig-style APIs to explicitly indicate
// "clear all existing nodes for the config", as opposed to "no nodes provided".
var ClearNodes []Node
