#!/usr/bin/env python3
"""Generate closed SDK schemas from the snapshot contract. No network access."""
import copy
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
out = root / "schemas"
out.mkdir(exist_ok=True)
draft = "https://json-schema.org/draft/2020-12/schema"

def obj(properties, required=()):
    return {"$schema": draft, "type": "object", "additionalProperties": False,
            "properties": properties, "required": list(required)}

def string(limit=256):
    return {"type": "string", "maxLength": limit}

def save(name, value):
    (out / (name + ".json")).write_text(json.dumps(value, indent=2) + "\n")

s = json.loads((root / "internal/snapshot/snapshot.schema.json").read_text())
s.pop("$id", None)
s["$defs"]["dependency"]["properties"]["serviceId"] = {"$ref": "#/$defs/id"}
save("snapshot", s)
meta = {key: copy.deepcopy(s["properties"][key]) for key in ("repository", "producer", "coverage", "completedAt")}
filters = {"search": string(), "repository": string(),
           "assessment": {"enum": ["", "updates_found", "no_candidate", "excluded", "unknown"]},
           "updateType": {"enum": ["", "major", "minor", "patch", "digest", "pinDigest", "pin", "unknown"]},
           "workflow": {"enum": ["", "pr_open", "branch_only", "none_observed", "unknown"]}}
save("summary-input", obj(filters))
save("list-input", obj(dict(filters, cursor=string(1024), revision=string(24), limit={"type": "integer", "minimum": 1, "maximum": 100})))
save("get-input", obj({"dependencyId": {"$ref": "snapshot.json#/$defs/id"}}, ["dependencyId"]))

def output(name, extra):
    value = obj(dict(meta, **extra), list(meta) + list(extra))
    value["$defs"] = s["$defs"]
    save(name, value)

output("list-output", {"items": {"type": "array", "maxItems": 100, "items": {"$ref": "#/$defs/dependency"}},
                       "total": {"$ref": "#/$defs/count"}, "revision": {"type": "string", "pattern": "^[1-9][0-9]*$"}, "nextCursor": string(1024)})
output("get-output", {"dependency": {"$ref": "#/$defs/dependency"}})
counts = {"type": "object", "additionalProperties": {"type": "integer", "minimum": 0, "maximum": 200000}}
output("summary-output", {"assessments": counts, "updateTypes": counts, "candidates": {"type": "integer", "minimum": 0, "maximum": 200000}})
save("widget", obj({"repository": string(), "updateType": dict(filters["updateType"], type="string", maxLength=32), "maxRows": {"type": "integer", "minimum": 1, "maximum": 5}}))
save("connection", obj({
    "snapshotURL": {"type": "string", "maxLength": 2048},
    "browserRepositoryURL": {"type": "string", "maxLength": 512},
    "intervalSeconds": {"type": "integer", "minimum": 30, "maximum": 900, "default": 60},
    "maxSnapshotAgeSeconds": {"type": "integer", "minimum": 3600, "maximum": 604800, "default": 43200},
    "notificationsEnabled": {"type": "boolean", "default": False},
    "serviceMappingsJSON": {"type": "string", "maxLength": 50000, "description": "Optional JSON array of exact dependencyId/serviceId mappings, at most 256."}
}, ["snapshotURL", "browserRepositoryURL"]))
