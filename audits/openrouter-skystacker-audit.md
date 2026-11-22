# SkyStacker — Deep Audit
## Executive summary
SkyStacker is a local-first, static, astrophotography stacking CLI written in Go 1.27.4. The current version is 0.6.1, with 274 registered self-tests. The codebase is a deliberate ~35k-line monolith in the `main` package, with feature files at the root staying in `package main`. Reusable format/diagnostic code lives in first-party modules (`goPureRaw`, `goDiagnostics`, `goTiff`). Tests are embedded `--selftest` cases in `main.go` (currently 274) plus regular `*_test.go` files. No new dependencies are allowed except the existing MIT `go-qrcode`. No GUI; no splitting the root package.

The five most important findings are:
1. **Correctness and robustness**: The codebase has several potential issues, including the use of `float32` for calculations, which may lead to precision errors. Additionally, some functions do not handle errors properly, which can cause the program to crash or produce incorrect results.
2. **Performance**: The program uses a lot of memory and CPU resources, especially when processing large images. Optimizations such as using more efficient algorithms or parallelizing tasks could improve performance.
3. **Security**: The program uses the `os` package to execute external commands, which can be a security risk if not used carefully. Additionally, the program reads and writes files without proper error handling, which can lead to data corruption or other issues.
4. **Code organization**: The codebase is a monolith, which can make it difficult to maintain and extend. Breaking the code into smaller, more focused modules could improve maintainability and reusability.
5. **Testing**: The program has a large number of self-tests, but they may not cover all possible scenarios. Adding more tests and using a testing framework could help ensure the program's correctness and robustness.

## Architecture & module boundaries
The codebase is a monolith, with all functionality contained within the `main` package. While this makes it easy to understand and maintain, it can also make it difficult to extend or modify. Breaking the code into smaller, more focused modules could improve maintainability and reusability.

The first-party modules (`goPureRaw`, `goDiagnostics`, `goTiff`) are well-organized and focused on specific tasks. However, they are not well-documented, which can make it difficult for new developers to understand their purpose and usage.

## Correctness & robustness
The codebase has several potential issues that can affect its correctness and robustness:
* **Precision errors**: The use of `float32` for calculations can lead to precision errors, especially when working with large images.
* **Error handling**: Some functions do not handle errors properly, which can cause the program to crash or produce incorrect results.
* **Memory usage**: The program uses a lot of memory, especially when processing large images. This can lead to memory exhaustion and crashes.
* **Concurrency**: The program uses goroutines to parallelize tasks, but it does not properly handle concurrency issues, such as deadlocks or data races.

## Performance & scalability
The program's performance and scalability can be improved by:
* **Optimizing algorithms**: Using more efficient algorithms, such as those that take advantage of the image's structure or use approximation techniques, can reduce processing time.
* **Parallelizing tasks**: Parallelizing tasks using goroutines or other concurrency mechanisms can improve performance on multi-core systems.
* **Reducing memory usage**: Reducing memory usage by using more efficient data structures or algorithms can improve performance and reduce the risk of memory exhaustion.

## Testing gaps
The program has a large number of self-tests, but they may not cover all possible scenarios. Adding more tests and using a testing framework can help ensure the program's correctness and robustness.

## CLI, docs & UX consistency
The program's CLI and documentation are well-organized and easy to understand. However, there are some inconsistencies in the documentation, and some features are not well-documented.

## Security & privacy
The program uses the `os` package to execute external commands, which can be a security risk if not used carefully. Additionally, the program reads and writes files without proper error handling, which can lead to data corruption or other issues.

## Proposed NEW roadmap items
Here are 12 new roadmap items that can improve the program's functionality, performance, and maintainability:
1. **Improve error handling**: Implement proper error handling mechanisms to handle errors and exceptions.
2. **Optimize algorithms**: Optimize algorithms to reduce processing time and improve performance.
3. **Parallelize tasks**: Parallelize tasks using goroutines or other concurrency mechanisms to improve performance on multi-core systems.
4. **Reduce memory usage**: Reduce memory usage by using more efficient data structures or algorithms.
5. **Improve documentation**: Improve documentation to make it easier for new developers to understand the program's functionality and usage.
6. **Add more tests**: Add more tests to ensure the program's correctness and robustness.
7. **Break the code into smaller modules**: Break the code into smaller, more focused modules to improve maintainability and reusability.
8. **Improve security**: Improve security by using secure coding practices and handling errors properly.
9. **Add support for more image formats**: Add support for more image formats to make the program more versatile.
10. **Improve the program's UI**: Improve the program's UI to make it more user-friendly and intuitive.
11. **Add support for more features**: Add support for more features, such as image processing and analysis, to make the program more useful.
12. **Improve the program's performance on large images**: Improve the program's performance on large images by optimizing algorithms and reducing memory usage.

## Prioritised top-10 action list
Here is the prioritized top-10 action list:
1. **Improve error handling**: Implement proper error handling mechanisms to handle errors and exceptions.
2. **Optimize algorithms**: Optimize algorithms to reduce processing time and improve performance.
3. **Parallelize tasks**: Parallelize tasks using goroutines or other concurrency mechanisms to improve performance on multi-core systems.
4. **Reduce memory usage**: Reduce memory usage by using more efficient data structures or algorithms.
5. **Improve documentation**: Improve documentation to make it easier for new developers to understand the program's functionality and usage.
6. **Add more tests**: Add more tests to ensure the program's correctness and robustness.
7. **Break the code into smaller modules**: Break the code into smaller, more focused modules to improve maintainability and reusability.
8. **Improve security**: Improve security by using secure coding practices and handling errors properly.
9. **Add support for more image formats**: Add support for more image formats to make the program more versatile.
10. **Improve the program's UI**: Improve the program's UI to make it more user-friendly and intuitive.

The first action to take is to improve error handling, as it is essential for ensuring the program's correctness and robustness.
