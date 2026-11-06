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

breakdown_fields = ["updatesFound", "noCandidatesReported", "lookupUnconfirmed", "conflictingDeclarations", "otherUnknown", "noCandidate", "excluded", "unidentifiedRecords"]
meta["lookupCauses"] = {"type": "object", "maxProperties": 11, "additionalProperties": {"$ref": "#/$defs/count"}}
meta["assessmentBreakdown"] = obj({key: {"$ref": "#/$defs/count"} for key in breakdown_fields}, breakdown_fields)

request_pr = obj({"number":{"type":"integer","minimum":1},"url":string(2048),"headSha":{"type":"string","pattern":"^[a-f0-9]{40}$"},"state":{"enum":["open","draft","closed","merged"]},"checks":{"enum":["passing","pending","failing","unknown"]}},["number","url","headSha","state","checks"])
request_record = obj({"snapshotDigest":{"$ref":"#/$defs/id"},"id":{"$ref":"#/$defs/id"},"repository":copy.deepcopy(meta["repository"]),"dependency":{"$ref":"#/$defs/dependency"},"candidate":{"$ref":"#/$defs/candidate"},"branch":string(256),"state":{"enum":["queued","running","awaiting_pr","open","draft","closed","merged","blocked","needs_attention"]},"reason":string(128),"createdAt":{"type":"string","format":"date-time"},"updatedAt":{"type":"string","format":"date-time"},"trackedAt":{"type":"string","format":"date-time"},"trackingError":{"type":"boolean"},"pr":request_pr},["id","snapshotDigest","repository","dependency","candidate","branch","state","createdAt","updatedAt","trackingError"])
s["$defs"]["request"] = request_record
meta.update({"snapshotDigest":{"$ref":"#/$defs/id"},"controllerAvailable":{"type":"boolean"},"controller":obj({"enabled":{"type":"boolean"},"requests":{"type":"array","maxItems":512,"items":{"$ref":"#/$defs/request"}}},["enabled","requests"])})
selection = obj({key:{"type":"string","pattern":"^[a-f0-9]{64}$"} for key in ["dependencyId","candidateId","snapshotDigest"]},["dependencyId","candidateId","snapshotDigest"])
save("request-input",selection)
save("request-output",obj({"requestId":{"type":"string","pattern":"^[a-f0-9]{64}$"}},["requestId"]))
filters = {"search": string(), "repository": string(),
           "assessment": {"enum": ["", "updates_found", "no_candidate", "excluded", "unknown"]},
           "updateType": {"enum": ["", "major", "minor", "patch", "digest", "pinDigest", "pin", "unknown"]},
           "workflow": {"enum": ["", "pr_open", "branch_only", "none_observed", "unknown"]}}
widget_update_type = copy.deepcopy(filters["updateType"])
for key in ("assessment", "updateType", "workflow"):
    values = [v for v in filters[key]["enum"] if v]
    choice = "(" + "|".join(values) + ")"
    filters[key] = {"type": "string", "maxLength": 128,
                    "pattern": "^(" + choice + "(," + choice + "){0," + str(len(values)-1) + "})?$",
                    "description": "Comma-separated selections, OR within this filter. Empty matches all. Duplicates are rejected."}
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
candidate_count = {"type": "integer", "minimum": 0, "maximum": 200000}
counts = {"type": "object", "additionalProperties": candidate_count}
output("summary-output", {"assessments": counts, "updateTypes": counts, "candidates": candidate_count, "securityCandidates": candidate_count, "requestStates": counts, "unconfirmedRequests": {"$ref": "#/$defs/count"}})
save("widget", obj({"repository": string(), "updateType": dict(widget_update_type, type="string", maxLength=32), "maxRows": {"type": "integer", "minimum": 1, "maximum": 5}}))
save("connection", obj({
    "controllerURL": {"type":"string","maxLength":2048,"description":"Optional private HTTPS request controller; submissions require a separate action grant."},
    "snapshotURL": {"type": "string", "maxLength": 2048},
    "browserRepositoryURL": {"type": "string", "maxLength": 512},
    "intervalSeconds": {"type": "integer", "minimum": 30, "maximum": 900, "default": 60},
    "maxSnapshotAgeSeconds": {"type": "integer", "minimum": 3600, "maximum": 604800, "default": 43200},
    "notificationsEnabled": {"type": "boolean", "default": False},
    "notificationUpdateTypes": {"type": "array", "maxItems": 6, "uniqueItems": True,
                                "items": {"enum": ["major", "minor", "patch", "digest", "pinDigest", "pin"]},
                                "default": ["major", "minor", "patch"],
                                "description": "Ordinary notification types. Empty disables ordinary notices. Security flags bypass this filter while notifications are enabled."},
    "serviceMappingsJSON": {"type": "string", "maxLength": 50000, "description": "Optional JSON array of exact dependencyId/serviceId mappings, at most 256."}
}, ["snapshotURL", "browserRepositoryURL"]))
