# test-label-analyzer

This tool has two main use cases
* generate stats about what tests are in a certain category
* given certain categories generate a string that can be used directly with [Ginkgo] `--filter` or `--skip` flags

Both use cases support input files that define a set of regular expressions for test names to match a certain category.

## generate stats about what tests are in a certain category

Say we want to know about how many tests of a given set (i.e. directory or file set) are in a certain category. We provide a configuration file to define what labels (either inside the test name or as an explicit [Ginkgo label]) match a certain category.

The tool then prints an overview of how many tests are in each category, additionally it prints out a list of all test names including their attributes as where to find each test inside the code base.

Example:

```sh
$ # create an output directory
$ mkdir -p /tmp/ginkgo-outlines
$ # generate outline data files from the ginkgo test files (those that contain an import from ginkgo)
$ for test_file in $(cd $ginkgo_test_dir && grep -l 'github.com/onsi/ginkgo/v2' ./*.go); do; ginkgo outline --format json $test_file > /tmp/ginkgo-outlines/${test_file//[\/\.]/_}.ginkgooutline.json ; done
$ # feed input files to test-label-analyzer to generate stats
$ test-label-analyzer stats --config-name quarantine \
    $(for outline_file in $(ls /tmp/ginkgo-outlines/); do; \
        echo " --test-outline-filepath /tmp/ginkgo-outlines/$outline_file" | \
        tr -d '\n'; done; echo "") \
        > /tmp/test-label-analyzer-output.json
$ # from the output we can generate the concatenated test names
$ jq '.MatchingSpecPathes[] | [ .[].text ] | join(" ")' /tmp/test-label-analyzer-output.json
```

## generate a string that can be used directly with [Ginkgo] `--filter` or `--skip` flags

[Ginkgo]: https://onsi.github.io/ginkgo/
[Ginkgo label]: https://onsi.github.io/ginkgo/#spec-labels
