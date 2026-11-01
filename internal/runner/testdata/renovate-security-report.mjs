// Synthetic candidate, serialized by the pinned engine's actual report writer.
import { addExtractionStats, addRepositoryStatus, exportStats } from '/usr/local/renovate/dist/instrumentation/reporting.js';
import { runBranchSummary } from '/usr/local/renovate/dist/workers/repository/finalize/repository-statistics.js';
import { setCache } from '/usr/local/renovate/dist/util/cache/repository/index.js';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { readFile } from 'node:fs/promises';
GlobalConfig.set({ localDir: '/tmp' });
const config = { reportType: 'file', reportPath: '/tmp/security-report.json', repository: 'example/read-only' };
addRepositoryStatus(config, 'activated');
addExtractionStats(config, { packageFiles: { kubernetes: [{ packageFile: 'a.yaml', deps: [{ depName: 'nginx', depType: 'container', datasource: 'docker', currentValue: '1', updates: [{ newValue: '2', updateType: 'major', branchName: 'renovate/nginx', isVulnerabilityAlert: true }] }] }] } });
setCache({ getData: () => ({ branches: [{ branchName: 'renovate/nginx', prNo: 42, result: process.argv[2] ?? 'needs-approval', upgrades: [{ manager: 'kubernetes', depType: 'container', packageFile: 'a.yaml', packageName: 'nginx', depName: 'nginx', updateType: 'major', currentValue: '1', newValue: '2', isVulnerabilityAlert: true }] }] }), isModified: () => false });
runBranchSummary(config);
await exportStats(config);
const report = await readFile(config.reportPath, 'utf8');
const branchUpgrade = JSON.parse(report).repositories['example/read-only'].branches[0].upgrades[0];
if ('manager' in branchUpgrade || 'depType' in branchUpgrade || 'isVulnerabilityAlert' in branchUpgrade) throw new Error('Pinned branch projection changed');
console.log('REPORT:' + report);
