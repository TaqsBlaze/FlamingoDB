# AGENTS.md

# FlamingoDB Development Guide

**Project:** FlamingoDB

**Language:** Go (>=1.24)

**Architecture:** Clean Architecture + Modular Design

**Goal:** Build a modern scientific database engine optimized for numerical computing, scientific datasets, geospatial information, multidimensional arrays, vectors, matrices, and large-scale analytical workloads.

---

# Project Philosophy

FlamingoDB is **not** intended to be another PostgreSQL or MySQL clone.

Instead, FlamingoDB should become a database that understands scientific data as a first-class citizen.

The codebase should always prioritize:

* Simplicity
* Readability
* Predictability
* Performance
* Extensibility
* Maintainability

A junior engineer should be able to understand most of the project structure without needing extensive documentation.

If something becomes overly clever, rewrite it to be simpler.

---

# Guiding Principles

## Keep it Simple

Prefer

* simple code
* explicit logic
* descriptive naming

Avoid

* unnecessary abstractions
* deep inheritance
* hidden behavior
* magic code

---

## Small Packages

Every package should have one responsibility.

Good

/storage
/parser
/pager
/index

Bad

/core

---

## Composition over Complexity

Favor small reusable components instead of giant classes or managers.

---

## Test Everything

Every package must have tests.

Every bug fix should include a regression test.

---

## Documentation First

Every exported function should have documentation.

Every package should contain a README when necessary.

---

# Architecture

```
Client

     │

SQL

     │

Lexer

     │

Parser

     │

AST

     │

Planner

     │

Optimizer

     │

Executor

     │

Storage Engine

     │

Pager

     │

Database File
```

Every layer only communicates with adjacent layers.

No shortcuts.

No package should directly access internals of another layer.

---

# Project Structure

```
flamingodb/

cmd/
    flamingodbd/
    flamingo/

internal/

    parser/
        lexer/
        parser/
        ast/

    planner/

    optimizer/

    executor/

    storage/
        pager/
        page/
        record/
        table/
        catalog/

    wal/

    transaction/

    index/
        btree/

    datatypes/
        numeric/
        vector/
        matrix/
        array/
        geo/

    functions/

    network/

pkg/

sdk/

docs/

tests/

scripts/
```

---

# Naming Rules

Good

```
ReadPage()

CreateTable()

InsertRecord()

Vector3
```

Bad

```
Do()

Manager()

Handle()

Stuff()
```

Names should describe intent.

---

# Error Handling

Never panic unless there is a programming bug.

Always return descriptive errors.

```
page not found

table does not exist

invalid page header

unexpected token
```

---

# Formatting

Always run

```
gofmt
```

before committing.

Also run

```
go test ./...
```

No commit should fail tests.

---

# Development Phases

---

# Phase 1

Foundation

Goal

Create a minimal working database.

Requirements

* project structure
* logger
* configuration
* database file
* page abstraction
* pager
* binary serialization
* simple table storage

Deliverables

* open database
* create database
* close database
* page manager

---

# Phase 2

Storage Engine

Requirements

* row format
* schema
* metadata
* catalog
* table manager

Support

* create table
* insert
* read

---

# Phase 3

SQL Lexer

Requirements

Tokenize

CREATE

TABLE

SELECT

INSERT

DELETE

UPDATE

WHERE

VALUES

Identifiers

Strings

Numbers

Operators

---

# Phase 4

Parser

Generate AST

```
SELECT

FROM

WHERE

INSERT

UPDATE

DELETE
```

---

# Phase 5

Planner

Convert AST

↓

Logical Plan

---

# Phase 6

Executor

Execute

* scans

* filters

* inserts

* updates

* deletes

---

# Phase 7

Indexes

Implement

B+ Tree

Support

CREATE INDEX

Lookup

Range Scan

---

# Phase 8

Transactions

Implement

* WAL
* rollback
* commit
* recovery

---

# Phase 9

Scientific Types

Native support

INT

FLOAT32

FLOAT64

FLOAT128

BIGFLOAT

COMPLEX

VECTOR2

VECTOR3

VECTOR4

ARRAY

MATRIX

TENSOR

---

# Phase 10

Scientific Functions

Implement

SIN

COS

TAN

LOG

EXP

SQRT

DOT

CROSS

MAGNITUDE

DISTANCE

---

# Phase 11

Geospatial

Support

POINT

LINESTRING

POLYGON

MULTIPOLYGON

Functions

AREA

BUFFER

DISTANCE

INTERSECTS

---

# Phase 12

Optimization

Implement

Query optimizer

Statistics

Cost estimation

Parallel execution

SIMD

Compression

---

# Phase 13

Networking

Database Server

TCP

HTTP

Connection Pool

Authentication

---

# Phase 14

Python SDK

Native SDK

```
import flamingodb
```

Support

queries

transactions

bulk insert

scientific arrays

---

# Development Rules

Never skip phases.

Never introduce future dependencies.

Every phase must be stable before moving forward.

---

# AI Agent Coordination

This repository is designed to be developed by **two AI agents working concurrently**.

Both agents MUST coordinate through shared documentation.

Neither agent may assume what the other is doing.

---

# Agent Roles

## Agent Alpha

Responsible for

* Storage Engine
* Pager
* Database File
* WAL
* Transactions
* Indexes
* Performance
* Serialization

Directories

```
internal/storage/

internal/index/

internal/wal/

internal/transaction/
```

---

## Agent Beta

Responsible for

* SQL
* Lexer
* Parser
* Planner
* Executor
* Scientific Types
* Functions
* SDK

Directories

```
internal/parser/

internal/planner/

internal/executor/

internal/functions/

internal/datatypes/

sdk/
```

---

# Shared Ownership

The following require discussion before changes.

```
AST

Public Interfaces

Package Layout

Configuration

Documentation

Database File Format
```

No breaking changes without agreement.

---

# Shared Memory

The repository must contain

```
docs/

    MEMORY.md

    PROGRESS.md

    DECISIONS.md

    ROADMAP.md

    TODO.md
```

These files are the shared memory for both agents.

---

# MEMORY.md

Contains

Current architecture

Current interfaces

Current invariants

Current assumptions

Any active refactoring

Both agents must read this file before beginning work.

---

# PROGRESS.md

Every completed task must be recorded.

Format

```
## YYYY-MM-DD

Completed

- Pager
- Page Header
- Binary Encoder

Next

- Record Format
```

---

# DECISIONS.md

Every architectural decision must be documented.

Example

```
Decision

Page Size = 8192 bytes

Reason

Better cache locality.

Status

Accepted
```

Never silently change architecture.

---

# TODO.md

Tracks

Pending tasks

Blocked tasks

Future work

---

# ROADMAP.md

Contains

Current phase

Next phase

Completion percentage

Milestones

---

# Synchronization Protocol

Before starting work

Both agents MUST

1. Pull latest changes
2. Read all shared documentation
3. Update local understanding
4. Check TODO.md
5. Verify no ownership conflicts

Before committing

Both agents MUST

1. Update PROGRESS.md
2. Update TODO.md
3. Update MEMORY.md if interfaces changed
4. Update DECISIONS.md if architecture changed
5. Run tests
6. Format code

---

# Conflict Prevention

An agent MUST NOT

* modify another agent's package
* rename public interfaces without documentation
* change page format without approval
* change AST without documenting it
* remove exported APIs without agreement

---

# Code Reviews

Every completed feature should satisfy

✓ Simple

✓ Tested

✓ Documented

✓ Formatted

✓ Backwards compatible

---

# Definition of Done

A task is complete only if

* Code compiles
* Tests pass
* Documentation updated
* Progress recorded
* TODO updated
* No conflicts introduced
* Public interfaces documented

---

# Long-Term Vision

FlamingoDB should become a scientific database capable of powering

* Climate modelling
* GIS systems
* Scientific research
* Engineering simulations
* Astronomy
* Physics
* Sensor networks
* Machine learning datasets
* Large numerical workloads

The codebase should remain understandable, modular, and maintainable even after years of development.

**Final Principle**

> "Every line of code should make FlamingoDB easier to understand, not harder."
>
>   │
>
> SQL
