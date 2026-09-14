"""The reviewed N09 matrix must satisfy its versioned structural contract."""
import json
import unittest
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parents[3]


class N09SchemaTests(unittest.TestCase):
    def test_review_matrix_contract(self):
        schema = json.loads((ROOT / 'packages/contracts/personal-acceptance-baseline.v2.schema.json').read_text())
        matrix = json.loads((ROOT / 'docs/evidence/personal-experience/n09-independent-review-20260914/matrix.json').read_text())
        Draft202012Validator.check_schema(schema)
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        validator.validate(matrix)
        matrix['schema_version'] = 'personal-acceptance-baseline/v1'
        self.assertTrue(list(validator.iter_errors(matrix)))
