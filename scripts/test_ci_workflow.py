"""Exercise the aggregate shell gate directly, without third-party YAML dependencies."""
import os
from pathlib import Path
import subprocess
import unittest

class AggregateGateTests(unittest.TestCase):
    def test_success_failure_and_skips(self):
        text = (Path(__file__).resolve().parents[1] / '.github/workflows/ci.yml').read_text()
        gate = text.split('  verify:', 1)[1].split('        run: |', 1)[1]
        script = '\n'.join(line[10:] if line.startswith('          ') else line for line in gate.splitlines())
        cases = [(True,'success','skipped','skipped','success',True),
                 (False,'success','success','success','success',True),
                 (False,'success','failure','skipped','success',False),
                 (True,'failure','skipped','skipped','skipped',False),
                 (True,'success','skipped','skipped','failure',False)]
        for docs,changes,go,docker,links,passed in cases:
            env=dict(os.environ,DOCS_ONLY=str(docs).lower(),CHANGES=changes,GO=go,DOCKER=docker,DOCS=links)
            result=subprocess.run(['/bin/sh','-c',script],env=env,capture_output=True,text=True,timeout=10)
            self.assertEqual(result.returncode == 0,passed,result.stdout+result.stderr)

if __name__ == '__main__': unittest.main()
