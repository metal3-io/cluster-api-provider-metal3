#!/usr/bin/env python3
#
# remove security contexts from stdin-read yaml, and output yaml to stdout
# this allows tilt's live update to work
#

import sys
import yaml


def main():
    # remove security contexts
    output = []
    content = "\n".join(sys.stdin.readlines())
    data = yaml.safe_load_all(content)

    for d in data:
        if d.get("kind", "") == "Deployment":
            try:
                spec = d["spec"]["template"]["spec"]
                spec["securityContext"] = {}
                for container in spec.get("containers", []):
                    container["securityContext"] = {}
            except (KeyError, TypeError):
                pass
        output.append(yaml.safe_dump(d))

    print("---\n".join(output))


if __name__ == "__main__":
    main()