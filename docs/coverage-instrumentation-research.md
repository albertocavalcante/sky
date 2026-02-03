# Code Coverage Instrumentation: Cross-Language Research

This document surveys code coverage instrumentation mechanisms across major programming
languages. It is intended to inform the design of a coverage API for starlark-go-x.

---

## Table of Contents

1. [Python (coverage.py, sys.settrace, sys.monitoring)](#python)
2. [JavaScript/Node.js (V8 coverage, Istanbul/nyc)](#javascript-nodejs)
3. [TypeScript (Source Maps)](#typescript)
4. [Java (JaCoCo)](#java-jacoco)
5. [Kotlin (Kover)](#kotlin-kover)
6. [Go (go test -cover, runtime/coverage)](#go)
7. [C++ (gcov, llvm-cov)](#c-gcov-llvm-cov)
8. [Key Takeaways for Starlark Coverage Design](#key-takeaways)

---

## Python

### How Coverage is Enabled

Python coverage can be enabled through multiple mechanisms:

```bash
# Command line via coverage.py
coverage run my_script.py
coverage run -m pytest

# Programmatic API
import coverage
cov = coverage.Coverage()
cov.start()
# ... run code ...
cov.stop()
cov.save()

# Environment variable (for subprocess coverage)
COVERAGE_PROCESS_START=.coveragerc python my_script.py
```

### Instrumentation Mechanism

Coverage.py uses **runtime tracing** with three different "cores":

| Core      | Mechanism                          | Python Version | Default          |
| --------- | ---------------------------------- | -------------- | ---------------- |
| `ctrace`  | C implementation of `sys.settrace` | All            | 3.9-3.13         |
| `pytrace` | Pure Python `sys.settrace`         | All            | Never (fallback) |
| `sysmon`  | `sys.monitoring` (PEP 669)         | 3.12+          | 3.14+            |

**sys.settrace Mechanism:**

- The Python interpreter invokes a trace function for each executed line
- Events: `'call'`, `'line'`, `'return'`, `'exception'`, `'opcode'`
- The trace function receives `(frame, event, arg)` and returns a local trace function

**sys.monitoring (PEP 669) Mechanism:**

- Low-impact monitoring API introduced in Python 3.12
- Events can be individually disabled after capture for better performance
- Supports LINE, JUMP, BRANCH events for coverage tools

### Key APIs/Interfaces

#### sys.settrace Callback Signature

```python
def trace_function(frame, event, arg):
    """
    Args:
        frame: Current stack frame object
        event: One of 'call', 'line', 'return', 'exception', 'opcode'
        arg: Event-specific data (None for line, return value for return, etc.)

    Returns:
        A trace function for local tracing, or None to disable
    """
    if event == 'call':
        # Called when entering a function
        return trace_function  # Return self to trace this scope
    elif event == 'line':
        # Called before executing each line
        filename = frame.f_code.co_filename
        lineno = frame.f_lineno
        record_coverage(filename, lineno)
        return trace_function
    return None

# Install the trace function
import sys
sys.settrace(trace_function)
```

#### sys.monitoring API (Python 3.12+)

```python
import sys

TOOL_ID = sys.monitoring.COVERAGE_ID  # Pre-defined: 1

# Register for events
sys.monitoring.set_events(TOOL_ID,
    sys.monitoring.events.LINE |
    sys.monitoring.events.BRANCH |
    sys.monitoring.events.JUMP
)

# Register callback
def line_callback(code, line_number):
    record_coverage(code.co_filename, line_number)
    return sys.monitoring.DISABLE  # Disable after first hit

sys.monitoring.register_callback(TOOL_ID, sys.monitoring.events.LINE, line_callback)
```

#### coverage.py Programmatic API

```python
import coverage

# Create coverage object
cov = coverage.Coverage(
    branch=True,           # Enable branch coverage
    source=['mypackage'],  # Limit to specific packages
    omit=['*/tests/*'],    # Exclude patterns
)

# Collect coverage
cov.start()
run_tests()
cov.stop()
cov.save()

# Generate reports
cov.report()                    # Text report to stdout
cov.html_report(directory='htmlcov')
cov.xml_report(outfile='coverage.xml')
cov.json_report(outfile='coverage.json')

# Access raw data
data = cov.get_data()
for filename in data.measured_files():
    lines = data.lines(filename)  # Set of executed line numbers
    arcs = data.arcs(filename)    # List of (from_line, to_line) for branches
```

### Data Collection and Reporting

Coverage data is stored in a **SQLite database** (`.coverage` file) containing:

- File paths
- Executed line numbers
- Arc (branch) data as `(previous_line, current_line)` pairs
- Execution contexts (for context-based coverage)

### Official Documentation

- [coverage.py Documentation](https://coverage.readthedocs.io/)
- [How coverage.py Works](https://coverage.readthedocs.io/en/latest/howitworks.html)
- [Python sys.settrace Documentation](https://docs.python.org/3/library/sys.html#sys.settrace)
- [PEP 669 - Low Impact Monitoring](https://peps.python.org/pep-0669/)
- [sys.monitoring Documentation](https://docs.python.org/3/library/sys.monitoring.html)

### Implementation Source Code

- **coverage.py Repository**: [github.com/nedbat/coveragepy](https://github.com/nedbat/coveragepy)
- **Python Tracer**: [coverage/pytracer.py](https://github.com/nedbat/coveragepy/blob/master/coverage/pytracer.py)
- **C Tracer**: [coverage/ctracer/tracer.c](https://github.com/nedbat/coveragepy/blob/master/coverage/ctracer/tracer.c)

---

## JavaScript/Node.js

### How Coverage is Enabled

#### V8 Built-in Coverage (Node.js 10.12+)

```bash
# Environment variable triggers coverage output
NODE_V8_COVERAGE=./coverage node app.js

# Programmatic control
node --inspect app.js
# Then use Inspector Protocol
```

#### Istanbul/nyc

```bash
# Command line
nyc mocha tests/
nyc --reporter=html npm test

# Package.json script
{
  "scripts": {
    "test": "nyc mocha"
  }
}
```

### Instrumentation Mechanism

V8 provides **two coverage modes**:

| Mode            | Description                       | Performance    | Data Loss     |
| --------------- | --------------------------------- | -------------- | ------------- |
| **Best-effort** | Uses existing invocation counters | Minimal impact | Possible (GC) |
| **Precise**     | Prevents GC of feedback vectors   | Some overhead  | None          |

**V8 Implementation Details:**

- Reuses **invocation counters** from the Ignition interpreter
- Reuses **source range tracking** from `Function.prototype.toString`
- For block-level coverage, inserts `IncBlockCounter` bytecode instructions

**Istanbul/nyc Implementation:**

- **AST-based instrumentation** using Babel
- Transforms source code to add counter increments
- Intercepts `require()` to instrument on-the-fly

### Key APIs/Interfaces

#### V8 Inspector Protocol (Chrome DevTools Protocol)

```javascript
const inspector = require('inspector');
const session = new inspector.Session();
session.connect();

// Enable profiler
session.post('Profiler.enable');

// Start precise coverage collection
session.post('Profiler.startPreciseCoverage', {
    callCount: true,    // Collect execution counts
    detailed: true      // Block-level granularity
});

// Run your code here...

// Collect coverage data
session.post('Profiler.takePreciseCoverage', (err, { result }) => {
    // result is array of ScriptCoverage objects
    for (const script of result) {
        console.log(`Script: ${script.url}`);
        for (const func of script.functions) {
            console.log(`  Function: ${func.functionName}`);
            for (const range of func.ranges) {
                console.log(`    ${range.startOffset}-${range.endOffset}: ${range.count}`);
            }
        }
    }
});

// Stop coverage
session.post('Profiler.stopPreciseCoverage');
```

#### Node.js v8 Module API

```javascript
const v8 = require('v8');

// Write coverage on demand (requires NODE_V8_COVERAGE set)
v8.takeCoverage();

// Stop coverage collection
v8.stopCoverage();
```

#### Istanbul Instrumenter API

```javascript
const { createInstrumenter } = require('istanbul-lib-instrument');

const instrumenter = createInstrumenter({
    coverageVariable: '__coverage__',
    esModules: true,
    compact: true,
    produceSourceMap: true
});

// Instrument code
const instrumented = instrumenter.instrumentSync(
    sourceCode,
    filename,
    inputSourceMap
);

// Get coverage object for the file
const fileCoverage = instrumenter.lastFileCoverage();
```

#### Istanbul programVisitor (Babel Plugin)

```javascript
// For use in Babel plugins
const { programVisitor } = require('istanbul-lib-instrument');

module.exports = function(babel) {
    return {
        visitor: {
            Program: {
                enter(path, state) {
                    this.__coverage__ = programVisitor(
                        babel.types,
                        state.file.opts.filename,
                        { coverageVariable: '__coverage__' }
                    );
                    this.__coverage__.enter(path);
                },
                exit(path) {
                    const { fileCoverage } = this.__coverage__.exit(path);
                    // fileCoverage contains the coverage map for this file
                }
            }
        }
    };
};
```

### Data Collection and Reporting

**V8 Coverage Format:**

```json
{
    "result": [{
        "scriptId": "1",
        "url": "file:///app.js",
        "functions": [{
            "functionName": "main",
            "ranges": [
                { "startOffset": 0, "endOffset": 100, "count": 1 },
                { "startOffset": 50, "endOffset": 80, "count": 0 }
            ],
            "isBlockCoverage": true
        }]
    }]
}
```

**Istanbul Coverage Format (`__coverage__`):**

```json
{
    "/path/to/file.js": {
        "path": "/path/to/file.js",
        "statementMap": { "0": { "start": { "line": 1, "column": 0 }, "end": { "line": 1, "column": 10 } } },
        "fnMap": { "0": { "name": "main", "decl": {...}, "loc": {...} } },
        "branchMap": { "0": { "type": "if", "locations": [...] } },
        "s": { "0": 1 },  // Statement counts
        "f": { "0": 1 },  // Function counts
        "b": { "0": [1, 0] }  // Branch counts
    }
}
```

### Official Documentation

- [V8 JavaScript Code Coverage Blog Post](https://v8.dev/blog/javascript-code-coverage)
- [Chrome DevTools Protocol - Profiler Domain](https://chromedevtools.github.io/devtools-protocol/v8/Profiler/)
- [Node.js Inspector API](https://nodejs.org/api/inspector.html)
- [Istanbul.js](https://istanbul.js.org/)
- [nyc GitHub Repository](https://github.com/istanbuljs/nyc)

### Implementation Source Code

- **V8 Source**: [github.com/v8/v8](https://github.com/v8/v8)
- **nyc**: [github.com/istanbuljs/nyc](https://github.com/istanbuljs/nyc)
- **istanbul-lib-instrument**: [github.com/istanbuljs/istanbuljs/tree/main/packages/istanbul-lib-instrument](https://github.com/istanbuljs/istanbuljs/tree/main/packages/istanbul-lib-instrument)
- **babel-plugin-istanbul**: [github.com/istanbuljs/babel-plugin-istanbul](https://github.com/istanbuljs/babel-plugin-istanbul)
- **c8 (V8-based coverage)**: [github.com/bcoe/c8](https://github.com/bcoe/c8)

---

## TypeScript

### How Coverage is Enabled

TypeScript coverage uses the same tools as JavaScript, with source map support:

```bash
# Using nyc with TypeScript
nyc --extension .ts mocha --require ts-node/register tests/**/*.ts

# Using Jest with ts-jest
jest --coverage

# Using c8 (V8 native coverage)
c8 ts-node src/index.ts
```

### Instrumentation Mechanism

TypeScript coverage works through **source map remapping**:

1. TypeScript compiles to JavaScript (generating `.js.map` files)
2. JavaScript is instrumented or profiled
3. Coverage data is mapped back to TypeScript using source maps

**Key tsconfig.json options:**

```json
{
    "compilerOptions": {
        "sourceMap": true,           // Generate .js.map files
        "inlineSourceMap": true,     // Embed source map in .js (for Jest)
        "inlineSources": true,       // Include TS source in map
        "declaration": true,
        "declarationMap": true
    }
}
```

### Key APIs/Interfaces

#### nyc Configuration for TypeScript

```json
// package.json or .nycrc
{
    "nyc": {
        "extension": [".ts", ".tsx"],
        "include": ["src/**/*.ts"],
        "exclude": ["**/*.d.ts", "**/*.test.ts"],
        "reporter": ["text", "html", "lcov"],
        "sourceMap": true,
        "instrument": true
    }
}
```

#### Jest Configuration

```javascript
// jest.config.js
module.exports = {
    preset: 'ts-jest',
    collectCoverage: true,
    coverageProvider: 'v8',  // or 'babel'
    collectCoverageFrom: ['src/**/*.ts'],
    coverageReporters: ['text', 'lcov', 'html']
};
```

### Data Collection and Reporting

Source maps follow the **Source Map V3 specification**:

```json
{
    "version": 3,
    "file": "output.js",
    "sources": ["input.ts"],
    "sourcesContent": ["// original TypeScript..."],
    "names": ["identifier1", "identifier2"],
    "mappings": "AAAA,SAAS..."  // VLQ-encoded mappings
}
```

Coverage tools use `source-map` library to remap positions:

```javascript
const { SourceMapConsumer } = require('source-map');

const consumer = await new SourceMapConsumer(sourceMapJSON);
const original = consumer.originalPositionFor({
    line: jsLine,
    column: jsColumn
});
// original.source, original.line, original.column
```

### Official Documentation

- [TypeScript sourceMap Option](https://www.typescriptlang.org/tsconfig/sourceMap.html)
- [ts-jest Documentation](https://kulshekhar.github.io/ts-jest/)
- [Source Map V3 Specification](https://sourcemaps.info/spec.html)

### Implementation Source Code

- **remap-istanbul**: [github.com/SitePen/remap-istanbul](https://github.com/SitePen/remap-istanbul)
- **ts-jest**: [github.com/kulshekhar/ts-jest](https://github.com/kulshekhar/ts-jest)
- **source-map**: [github.com/nicolo-ribaudo/source-map-js](https://github.com/nicolo-ribaudo/source-map-js)

---

## Java (JaCoCo)

### How Coverage is Enabled

```bash
# Java agent (on-the-fly instrumentation)
java -javaagent:jacocoagent.jar=destfile=jacoco.exec \
     -jar myapp.jar

# With specific options
java -javaagent:jacocoagent.jar=destfile=jacoco.exec,\
includes=com.mycompany.*,excludes=*Test,output=file \
     -jar myapp.jar

# Maven
mvn test jacoco:report

# Gradle
./gradlew test jacocoTestReport
```

### Instrumentation Mechanism

JaCoCo uses **bytecode instrumentation** via the ASM library:

| Approach       | When          | Use Case                      |
| -------------- | ------------- | ----------------------------- |
| **On-the-fly** | Class loading | Standard usage via Java agent |
| **Offline**    | Build time    | Android, special classloaders |

**Probe Implementation:**

- Adds a `boolean[]` array (`$jacocoData`) to each class
- Adds a static method `$jacocoInit()` for initialization
- Each probe sets `array[index] = true` when executed
- ~30% class size increase, <10% runtime overhead

**Class Identity:**

- Uses CRC64 hash of raw class bytes
- Handles multi-classloader environments (OSGi, etc.)

### Key APIs/Interfaces

#### Java Agent Options

| Option       | Description                       | Default       |
| ------------ | --------------------------------- | ------------- |
| `destfile`   | Output file path                  | `jacoco.exec` |
| `append`     | Append to existing file           | `true`        |
| `includes`   | Classes to instrument (wildcards) | `*`           |
| `excludes`   | Classes to skip                   | (none)        |
| `output`     | `file`, `tcpserver`, `tcpclient`  | `file`        |
| `port`       | TCP port for server/client mode   | `6300`        |
| `dumponexit` | Dump on JVM shutdown              | `true`        |
| `jmx`        | Enable JMX MBean                  | `false`       |

#### JaCoCo Core API

```java
import org.jacoco.core.analysis.*;
import org.jacoco.core.data.*;
import org.jacoco.core.instr.*;

// Offline instrumentation
Instrumenter instrumenter = new Instrumenter(runtime);
byte[] instrumented = instrumenter.instrument(
    originalClassBytes,
    className
);

// Runtime data collection
RuntimeData data = new RuntimeData();
IRuntime runtime = new LoggerRuntime();
runtime.startup(data);

// After execution, collect coverage
ExecutionDataStore executionData = new ExecutionDataStore();
SessionInfoStore sessionInfo = new SessionInfoStore();
data.collect(executionData, sessionInfo, false);

// Analyze coverage
CoverageBuilder coverageBuilder = new CoverageBuilder();
Analyzer analyzer = new Analyzer(executionData, coverageBuilder);
analyzer.analyzeClass(originalClassBytes, className);

// Access results
for (IClassCoverage cc : coverageBuilder.getClasses()) {
    System.out.printf("Class: %s, Lines: %d/%d%n",
        cc.getName(),
        cc.getLineCounter().getCoveredCount(),
        cc.getLineCounter().getTotalCount());
}
```

#### Maven Configuration

```xml
<plugin>
    <groupId>org.jacoco</groupId>
    <artifactId>jacoco-maven-plugin</artifactId>
    <version>0.8.11</version>
    <executions>
        <execution>
            <goals>
                <goal>prepare-agent</goal>
            </goals>
        </execution>
        <execution>
            <id>report</id>
            <phase>test</phase>
            <goals>
                <goal>report</goal>
            </goals>
        </execution>
    </executions>
</plugin>
```

#### Gradle Configuration

```kotlin
plugins {
    jacoco
}

jacoco {
    toolVersion = "0.8.11"
}

tasks.test {
    finalizedBy(tasks.jacocoTestReport)
}

tasks.jacocoTestReport {
    dependsOn(tasks.test)
    reports {
        xml.required.set(true)
        html.required.set(true)
    }
}
```

### Data Collection and Reporting

JaCoCo uses a binary `.exec` format containing:

- Session information (ID, start time, dump time)
- Execution data per class (class ID, name, probe array)

Report formats: HTML, XML, CSV

### Official Documentation

- [JaCoCo Documentation](https://www.jacoco.org/jacoco/trunk/doc/)
- [JaCoCo Implementation Design](https://www.jacoco.org/jacoco/trunk/doc/implementation.html)
- [JaCoCo Java Agent](https://www.eclemma.org/jacoco/trunk/doc/agent.html)
- [JaCoCo Control Flow Analysis](https://www.jacoco.org/jacoco/trunk/doc/flow.html)

### Implementation Source Code

- **JaCoCo Repository**: [github.com/jacoco/jacoco](https://github.com/jacoco/jacoco)
- **Agent PreMain**: [org.jacoco.agent.rt/src/org/jacoco/agent/rt/internal/PreMain.java](https://github.com/jacoco/jacoco/blob/master/org.jacoco.agent.rt/src/org/jacoco/agent/rt/internal/PreMain.java)
- **Coverage Transformer**: [CoverageTransformer.java](https://github.com/jacoco/jacoco/blob/master/org.jacoco.agent.rt/src/org/jacoco/agent/rt/internal/CoverageTransformer.java)
- **Core Instrumenter**: [org.jacoco.core](https://github.com/jacoco/jacoco/tree/master/org.jacoco.core)

---

## Kotlin (Kover)

### How Coverage is Enabled

```kotlin
// build.gradle.kts
plugins {
    id("org.jetbrains.kotlinx.kover") version "0.9.5"
}

// Run coverage
// ./gradlew koverHtmlReport
// ./gradlew koverXmlReport
// ./gradlew koverVerify
```

### Instrumentation Mechanism

Kover supports **two coverage engines**:

| Engine       | Description                            | Use Case                  |
| ------------ | -------------------------------------- | ------------------------- |
| **IntelliJ** | Native Kotlin instrumentation          | Default, Kotlin-optimized |
| **JaCoCo**   | Standard Java bytecode instrumentation | Compatibility             |

**Note:** The Kotlin team is transitioning to using the JaCoCo JVM Agent for future versions.

### Key APIs/Interfaces

#### Gradle Plugin Configuration

```kotlin
// build.gradle.kts
kover {
    // Use JaCoCo instead of IntelliJ engine
    useJacoco()

    // Configure reports
    reports {
        total {
            xml {
                onCheck = true
                xmlFile = file("build/reports/kover/report.xml")
            }
            html {
                onCheck = true
                htmlDir = file("build/reports/kover/html")
            }
        }
    }

    // Verification rules
    verify {
        rule {
            minBound(80)  // Minimum 80% coverage
        }
    }
}
```

#### Filtering

```kotlin
kover {
    reports {
        filters {
            includes {
                classes("com.example.*")
            }
            excludes {
                classes("*.Generated*")
                annotatedBy("Generated")
            }
        }
    }
}
```

#### Multi-Module Configuration

```kotlin
// Root build.gradle.kts
plugins {
    id("org.jetbrains.kotlinx.kover") version "0.9.5"
}

dependencies {
    kover(project(":submodule1"))
    kover(project(":submodule2"))
}
```

### Data Collection and Reporting

Kover outputs:

- **IC format**: IntelliJ's internal binary format
- **JaCoCo-compatible XML**: For CI/CD integration
- **HTML reports**: Human-readable visualization

### Official Documentation

- [Kover Gradle Plugin Documentation](https://kotlin.github.io/kotlinx-kover/gradle-plugin/)
- [Kover GitHub Repository](https://github.com/Kotlin/kotlinx-kover)

### Implementation Source Code

- **Kover Repository**: [github.com/Kotlin/kotlinx-kover](https://github.com/Kotlin/kotlinx-kover)

---

## Go

### How Coverage is Enabled

```bash
# Unit test coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go test -covermode=count -coverprofile=coverage.out ./...

# Integration test coverage (Go 1.20+)
go build -cover -o myapp ./cmd/myapp
GOCOVERDIR=./coverage ./myapp
go tool covdata percent -i=./coverage
```

### Instrumentation Mechanism

Go uses **source-level rewriting** (not binary instrumentation):

1. `go tool cover` rewrites source files to add counter increments
2. The modified source is compiled normally
3. Counter values are collected at runtime

**Example transformation:**

```go
// Original
func Size(a int) string {
    if a < 0 {
        return "negative"
    }
    return "positive"
}

// Instrumented
func Size(a int) string {
    GoCover.Count[0] = 1
    if a < 0 {
        GoCover.Count[1] = 1
        return "negative"
    }
    GoCover.Count[2] = 1
    return "positive"
}
```

**Coverage Modes:**

| Mode     | Description           | Use Case         |
| -------- | --------------------- | ---------------- |
| `set`    | Boolean (did it run?) | Default, fastest |
| `count`  | Execution count       | Heat maps        |
| `atomic` | Thread-safe counts    | Concurrent code  |

### Key APIs/Interfaces

#### runtime/coverage Package (Go 1.20+)

```go
package main

import (
    "os"
    "runtime/coverage"
)

func main() {
    // For long-running applications

    // Write coverage meta-data
    err := coverage.WriteMetaDir("/tmp/coverage")
    if err != nil {
        // handle error
    }

    // Write coverage counters (snapshot)
    err = coverage.WriteCountersDir("/tmp/coverage")
    if err != nil {
        // handle error
    }

    // Reset counters (requires -covermode=atomic)
    err = coverage.ClearCounters()
    if err != nil {
        // handle error
    }

    // Or write to custom io.Writer
    coverage.WriteMeta(os.Stdout)
    coverage.WriteCounters(os.Stdout)
}
```

#### Command Line Tools

```bash
# Generate profile
go test -coverprofile=coverage.out ./...

# View coverage by function
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Integration test coverage (Go 1.20+)
go build -cover -coverpkg=./... -o myapp .
GOCOVERDIR=./coverdata ./myapp [args]
go tool covdata percent -i=./coverdata
go tool covdata textfmt -i=./coverdata -o=profile.txt
go tool covdata merge -i=dir1,dir2 -o=merged
```

#### Coverage Profile Format

```
mode: set
example.com/pkg/main.go:10.2,12.16 1 1
example.com/pkg/main.go:14.2,16.3 1 0
```

Format: `filename:startline.startcol,endline.endcol statements count`

### Data Collection and Reporting

Go 1.20+ produces two file types in `GOCOVERDIR`:

- **Meta-data files**: Source file names, function names (invariant)
- **Counter data files**: Execution counts (one per run)

### Official Documentation

- [The cover story (Go Blog)](https://go.dev/blog/cover)
- [Code coverage for Go integration tests](https://go.dev/blog/integration-test-coverage)
- [Coverage profiling support for integration tests](https://go.dev/doc/build-cover)
- [go tool cover](https://pkg.go.dev/cmd/cover)
- [runtime/coverage package](https://pkg.go.dev/runtime/coverage)

### Implementation Source Code

- **Go Repository**: [github.com/golang/go](https://github.com/golang/go)
- **cover tool**: [src/cmd/cover/cover.go](https://github.com/golang/go/blob/master/src/cmd/cover/cover.go)
- **runtime/coverage**: [src/runtime/coverage/coverage.go](https://cs.opensource.google/go/go/+/refs/tags/go1.25.6:src/runtime/coverage/coverage.go)
- **internal/coverage**: [src/internal/coverage](https://pkg.go.dev/internal/coverage)

---

## C++ (gcov, llvm-cov)

### How Coverage is Enabled

#### GCC (gcov)

```bash
# Compile with coverage flags
g++ -fprofile-arcs -ftest-coverage -O0 -g main.cpp -o main
# Or use shorthand
g++ --coverage -O0 -g main.cpp -o main

# Run the program
./main

# Generate coverage report
gcov main.cpp
# Or use gcovr for better reports
gcovr --html-details coverage.html
```

#### Clang/LLVM (llvm-cov)

```bash
# Source-based coverage (recommended)
clang++ -fprofile-instr-generate -fcoverage-mapping -O0 main.cpp -o main

# Run to generate raw profile
./main  # Creates default.profraw

# Process raw profile
llvm-profdata merge -sparse default.profraw -o main.profdata

# Generate reports
llvm-cov show ./main -instr-profile=main.profdata
llvm-cov report ./main -instr-profile=main.profdata
llvm-cov export ./main -instr-profile=main.profdata -format=lcov > coverage.lcov
```

### Instrumentation Mechanism

**GCC (gcov):**

- Compiler inserts counter increments at branch points
- Generates `.gcno` files (structure/notes) at compile time
- Generates `.gcda` files (data/counts) at runtime
- Counter data accumulated across runs

**LLVM/Clang:**

- **gcov-compatible mode**: Similar to GCC, uses `-fprofile-arcs -ftest-coverage`
- **Source-based coverage**: Uses `-fprofile-instr-generate -fcoverage-mapping`
  - More accurate mapping to source
  - Supports expression-level coverage
  - Supports MC/DC (Modified Condition/Decision Coverage)

### Key APIs/Interfaces

#### GCC Runtime Library Functions

```c
// Called automatically, but can be invoked manually
extern void __gcov_flush(void);  // Write coverage data
extern void __gcov_reset(void);  // Reset counters
extern void __gcov_dump(void);   // Write and reset (GCC 11+)
```

#### LLVM Profile Runtime

```c
// LLVM profile runtime functions
void __llvm_profile_initialize_file(void);
int __llvm_profile_write_file(void);
void __llvm_profile_reset_counters(void);
void __llvm_profile_set_filename(const char *);
const char *__llvm_profile_get_filename(void);
```

#### CMake Integration

```cmake
# GCC/gcov
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} --coverage")
set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} --coverage")

# Clang source-based coverage
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fprofile-instr-generate -fcoverage-mapping")
set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -fprofile-instr-generate")
```

### Data Collection and Reporting

**gcov output format (.gcov):**

```
    -:    1:#include <stdio.h>
    -:    2:
    1:    3:int main() {
    1:    4:    int x = 5;
    1:    5:    if (x > 0) {
    1:    6:        printf("positive\n");
    -:    7:    } else {
#####:    8:        printf("non-positive\n");
    -:    9:    }
    1:   10:    return 0;
    -:   11:}
```

**llvm-cov JSON export format:**

```json
{
    "data": [{
        "files": [{
            "filename": "main.cpp",
            "segments": [
                [1, 1, 10, true, true, false],
                [3, 5, 0, false, false, false]
            ],
            "summary": {
                "lines": { "count": 10, "covered": 8, "percent": 80.0 },
                "functions": { "count": 2, "covered": 2, "percent": 100.0 },
                "branches": { "count": 4, "covered": 3, "percent": 75.0 }
            }
        }]
    }],
    "type": "llvm.coverage.json.export",
    "version": "2.0.1"
}
```

### Official Documentation

- [GCC gcov Documentation](https://gcc.gnu.org/onlinedocs/gcc/Gcov.html)
- [LLVM Source-Based Code Coverage](https://clang.llvm.org/docs/SourceBasedCodeCoverage.html)
- [llvm-cov Command Guide](https://llvm.org/docs/CommandGuide/llvm-cov.html)
- [gcovr Documentation](https://gcovr.com/en/stable/)

### Implementation Source Code

- **GCC libgcov**: [github.com/gcc-mirror/gcc/tree/master/libgcc](https://github.com/gcc-mirror/gcc/tree/master/libgcc)
- **LLVM coverage**: [github.com/llvm/llvm-project/tree/main/compiler-rt/lib/profile](https://github.com/llvm/llvm-project/tree/main/compiler-rt/lib/profile)
- **llvm-cov tool**: [github.com/llvm/llvm-project/tree/main/llvm/tools/llvm-cov](https://github.com/llvm/llvm-project/tree/main/llvm/tools/llvm-cov)
- **gcovr**: [github.com/gcovr/gcovr](https://github.com/gcovr/gcovr)

---

## Key Takeaways for Starlark Coverage Design

### Common Patterns Across Languages

| Aspect                 | Pattern                                 | Examples                                                     |
| ---------------------- | --------------------------------------- | ------------------------------------------------------------ |
| **Enabling**           | Flag/env var at startup                 | Go `-cover`, Node `NODE_V8_COVERAGE`, Java `-javaagent`      |
| **Runtime API**        | Write/flush/reset counters              | Go `runtime/coverage`, C++ `__gcov_flush`, V8 `takeCoverage` |
| **Callback Interface** | Event-based hooks                       | Python `sys.settrace`, V8 Inspector Protocol                 |
| **Data Format**        | Line/offset + count pairs               | All languages                                                |
| **Granularity Levels** | Statement, branch, function, expression | Istanbul, JaCoCo, V8                                         |

### Recommended Starlark Coverage API Design

Based on this research, a Starlark coverage API should support:

#### 1. Enabling Coverage

```go
// Option 1: Thread option (like Go)
thread := &starlark.Thread{
    Name: "coverage",
}
thread.SetLocal("coverage", coverage.New())

// Option 2: Global runtime option (like Python)
starlark.SetCoverageEnabled(true)
```

#### 2. Callback/Hook Interface (like Python sys.settrace)

```go
type CoverageCallback interface {
    // Called when entering a statement
    OnStatement(file string, line, col int)

    // Called when entering/exiting a function
    OnCall(file string, line int, name string)
    OnReturn(file string, line int, name string)

    // Called for branch decisions
    OnBranch(file string, line int, taken bool)
}

thread.SetCoverageCallback(callback)
```

#### 3. Data Collection API (like Go runtime/coverage)

```go
type CoverageData interface {
    // Get executed lines per file
    Lines(file string) []int

    // Get execution counts per line
    Counts(file string) map[int]int

    // Get branch coverage (line -> [true_count, false_count])
    Branches(file string) map[int][2]int

    // Reset counters
    Clear()

    // Write to standard format
    WriteLCOV(w io.Writer) error
    WriteJSON(w io.Writer) error
}

data := thread.Coverage()
```

#### 4. Instrumentation Mode Selection (like Go covermode)

```go
type CoverageMode int

const (
    CoverageModeSet    CoverageMode = iota  // Boolean: was it executed?
    CoverageModeCount                        // Integer: how many times?
    CoverageModeAtomic                       // Thread-safe counts
)

coverage.SetMode(CoverageModeCount)
```

### Key Design Decisions

1. **Runtime vs Compile-time Instrumentation**
   - Python, JS/V8: Runtime tracing/hooks (flexible, some overhead)
   - Go: Source rewriting (fast, requires tooling)
   - Java: Bytecode manipulation (transparent, JVM-specific)
   - **Recommendation for Starlark**: Runtime hooks (simpler, Starlark is interpreted)

2. **Granularity**
   - Minimum: Statement/line coverage
   - Recommended: Add branch coverage (if statements, conditionals)
   - Advanced: Expression-level (V8 block coverage)

3. **Data Format**
   - Use established formats for compatibility: LCOV, Cobertura XML, Istanbul JSON
   - Store internally as `map[file]map[line]count`

4. **Performance Considerations**
   - Python's approach: Disable tracing after first hit for "set" mode
   - Go's approach: Simple counter increments (~3% overhead)
   - Consider optional "sampling" mode for production

---

## References Summary

### Python

- [coverage.py Documentation](https://coverage.readthedocs.io/)
- [coverage.py GitHub](https://github.com/nedbat/coveragepy)
- [PEP 669](https://peps.python.org/pep-0669/)

### JavaScript/Node.js

- [V8 Coverage Blog](https://v8.dev/blog/javascript-code-coverage)
- [Istanbul.js](https://istanbul.js.org/)
- [nyc GitHub](https://github.com/istanbuljs/nyc)
- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/v8/Profiler/)

### Java

- [JaCoCo Documentation](https://www.jacoco.org/jacoco/trunk/doc/)
- [JaCoCo GitHub](https://github.com/jacoco/jacoco)

### Kotlin

- [Kover Documentation](https://kotlin.github.io/kotlinx-kover/gradle-plugin/)
- [Kover GitHub](https://github.com/Kotlin/kotlinx-kover)

### Go

- [Go Coverage Blog Posts](https://go.dev/blog/cover)
- [runtime/coverage Package](https://pkg.go.dev/runtime/coverage)
- [Go Source Code](https://github.com/golang/go/blob/master/src/cmd/cover/cover.go)

### C++

- [LLVM Source-Based Coverage](https://clang.llvm.org/docs/SourceBasedCodeCoverage.html)
- [GCC gcov](https://gcc.gnu.org/onlinedocs/gcc/Gcov.html)
- [gcovr](https://gcovr.com/)
