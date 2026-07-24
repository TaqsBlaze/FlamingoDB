# FlamingoDB v1.3.0 "Plumage" Release Notes

**Release Date:** July 23, 2026

## Overview
FlamingoDB v1.3.0 "Plumage" introduces significant enhancements to AI-assisted database interactions, featuring expanded AI model support, improved Model Context Protocol (MCP) integration, and advanced data visualization capabilities.

## New Features

### 🤖 Expanded AI Model Support
- **Multi-provider AI integration**: Seamless connectivity to leading AI providers including:
  - OpenAI (ChatGPT GPT-4o, GPT-4o-mini, o1/o3 series)
  - Anthropic (Claude 3.5 Haiku and Sonnet)
  - Google (Gemini 1.5 Flash/Pro, 2.0 Flash)
  - DeepSeek (Chat V3 and Reasoner R1)
- **Configurable thinking levels**: Adjust reasoning effort for supported models (OpenAI o1/o3, Gemini 2.0 Thinking, DeepSeek R1)
- **Per-model policy assignment**: Assign custom security policies to each AI model configuration
- **Enhanced model management**: Improved UI for adding, editing, and configuring AI models in Settings

### 🔌 Model Context Protocol (MCP) Enhancements
- **Standardized tool interface**: Robust MCP implementation for secure AI-database communication
- **Expanded toolset**: 
  - `list_tables`: Discover available database tables
  - `describe_table`: Examine table schemas and column details
  - `execute_query`: Execute SQL queries with policy-based authorization
  - `generate_chart`: Create chart specifications for data visualization (new in this release)
- **Improved error handling**: Clearer permission messages and better tool execution feedback

### 📊 Advanced Data Visualization
- **AI-generated charts**: Natural language to chart generation via MCP `generate_chart` tool
- **Per-model chart persistence**: Each AI model maintains its own chart collection that persists across sessions and model switches
- **Interactive chart rendering**: 
  - Inline chart display within AI chat messages for immediate feedback
  - Dedicated persistent chart view that updates when switching models
  - Support for multiple chart types: bar, line, pie, doughnut, radar, polarArea, scatter, bubble
- **LocalStorage persistence**: Chart configurations survive page reloads and browser restarts

### 💬 AI Assistant Improvements
- **Enhanced chat persistence**: Conversation history automatically saved to localStorage
- **Model switching**: Seamless transition between AI models with preserved context
- **Message editing**: Ability to edit and resend messages for refined queries
- **Query integration**: Direct SQL execution from chat interface with results formatting
- **Improved UI/UX**: Refined chat interface with better loading states and message actions

## Technical Improvements
- **Performance optimizations**: Reduced chart rendering footprint for better chat window integration
- **Security enhancements**: Improved policy enforcement for MCP tool execution
- **Stability fixes**: Resolved various edge cases in chat history and chart state management
- **Code quality**: Updated dependencies and improved error logging throughout

## Getting Started
To use the new AI features:
1. Navigate to Settings → AI Models to configure your preferred AI providers
2. Add API keys for ChatGPT, Gemini, Claude, or DeepSeek as desired
3. Visit the AI Assistant page to start querying your database with natural language
4. Request visualizations by asking for charts - e.g., "Show me a bar chart of sales by region"
5. Switch between models to compare responses while preserving individual chart collections

## Compatibility
FlamingoDB v1.3.0 maintains backward compatibility with existing configurations and databases. No migration steps are required for upgrading from previous versions.

This release represents our commitment to making database interaction more intuitive, powerful, and accessible through AI-assisted tools while maintaining the security and reliability FlamingoDB is known for.