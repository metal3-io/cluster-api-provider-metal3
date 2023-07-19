# Fuzzing

 Fuzzing or fuzz testing is an automated software testing technique that
 involves providing invalid, unexpected, or random data as inputs to a
 computer program. A fuzzing target is defined for running the tests.
 These targets are defined in the test/fuzz directory and run using
 the instructions given below:

- Install the package from <https://github.com/mdempsky/go114-fuzz-build>
  using go get command. This is the link for the fuzzing tool that uses
  libfuzzer for its implementation purposes. I set it up and made a sample
  function to run the fuzzing tests. In these steps following are the important steps

   a.  Run below command to build the file for the function by giving its location.

    ```bash
    mkdir output
    cd output
    go114-fuzz-build -o FuzzTestImageValidate.a
    -func FuzzTestImageValidate ../test/fuzz/
    ```

    b. Run below command to make C binary files for the same function.

    ```
    clang -o FuzzTestImageValidate FuzzTestImageValidate.a
    -fsanitize=fuzzer
    ```

    c. Run the fuzzer as below

    ```
    /FuzzTestImageValidate
    ```

- Execution: Here is an example for execution of some test outputs:

    ```bash
    ./FuzzTestImageValidate
    INFO: Running with entropic power schedule (0xFF, 100).
    INFO: Seed: 376384327
    INFO: 267805 Extra Counters
    INFO: -max_len is not provided; libFuzzer will not generate inputs larger than 4096 bytes
    INFO: A corpus is not provided, starting from an empty corpus
    #2      INITED ft: 35 corp: 1/1b exec/s: 0 rss: 51Mb
    ```

    Here first line contains the byte array provided by libfuzzer to the
    function we then typecast it to our struct of IPPool using JSON.
    Unmarshal and then run the function. It keeps on running
    until it finds a bug.
