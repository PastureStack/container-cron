#!/usr/bin/env python3
"""Check stdin evidence before native VEX; never read a caller-selected path."""
import argparse
from collections import Counter
import hashlib
import json
import re
import sys

TARGET = 'usr/local/lib/docker/cli-plugins/docker-buildx'
PURL = 'pkg:golang/github.com/docker/docker@v28.5.2%2Bincompatible'
CVES = {'CVE-2026-41567', 'CVE-2026-42306'}
NAMES = 'github.com/docker/docker/pkg/namesgenerator'
MAIN = 'github.com/docker/buildx/cmd/buildx'
NAME_FILE = 'module/github.com/docker/docker@v28.5.2+incompatible/pkg/namesgenerator/names-generator.go'
NAME_SHA = 'cd2f9528ad47ae7519d78e6381e0b2313fa9b5ff541814e95e06ebb187aa9b64'
PINS = {
    'schemaVersion': '1', 'upstreamVersion': 'v0.38.1',
    'downstreamVersion': 'v0.38.1-pasturestack.1',
    'sourceCommit': '0dbe87f36ed472fe36b6796c3f70b4210a731fc5',
    'sourceArchiveSha256': 'c5c810b083ed3725f7d4299ba01505b5891d164cf81c3398ea4c3b27a3b71149',
    'originalGoModSha256': 'a93029abb94e88ddaa532ddeaab4802259a1025408f1eac417a9886e140067ef',
    'updatedGoModSha256': '230c98fa66a534b3a4d2dcdb0507e871658deff3c80058b01d85d7a18ce656b9',
    'goVersion': 'go1.27.3', 'goos': 'linux',
}
IDS = {'GO-2026-5617', 'GO-2026-5746', 'GO-2026-6253', 'GO-2026-6611'}
OSV_PINS = {
    'GO-2026-5617': '5a24060e5cc8cbf6a146615ebe5a7e5beac2f8f267c905538736ed6931ae2af5',
    'GO-2026-5746': 'd11da88232883a96ac3b74f848f172ce86c3783b7818d7da4b925335621d5273',
    'GO-2026-6253': '4f337e5aad0f8e83e3db2fc67d058fe0e80017b8c68f494b7e82be87e7120872',
    'GO-2026-6611': '5e89ac4314a24eac41111b71ef4544f22836c7b1baf48747e31c5c1d8282c3de',
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'Duplicate metadata key')
        result[key] = value
    return result


def load(text):
    return json.loads(text, object_pairs_hook=unique_pairs)


def objects(text):
    decoder = json.JSONDecoder(object_pairs_hook=unique_pairs)
    offset = 0
    while text[offset:].strip():
        offset += len(text[offset:]) - len(text[offset:].lstrip())
        item, offset = decoder.raw_decode(text, offset)
        yield item


def sha(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()


def findings(report):
    for result in report.get('Results', []):
        for finding in result.get('Vulnerabilities', []):
            yield result['Target'], finding


def pair(finding):
    return finding['VulnerabilityID'], finding['PkgIdentifier']['PURL']


def vex_pairs(document):
    pairs = []
    for statement in document['statements']:
        require(statement['status'] == 'not_affected'
                and statement['justification'] == 'vulnerable_code_not_present', 'Unexpected Dapper VEX status')
        pairs.extend((statement['vulnerability']['name'], p['@id']) for p in statement['products'])
    require(len(pairs) == len(set(pairs)), 'Duplicate VEX scope')
    return set(pairs)


def source_check(data):
    binding = unique_pairs(line.split('\t') for line in data['binding'].splitlines())
    require(set(binding) == set(PINS) | {'goarch', 'binarySha256'}, 'Incomplete source binding')
    require(all(binding[k] == v for k, v in PINS.items()), 'Wrong official source/toolchain')
    require(binding['goarch'] in ['amd64', 'arm64'], 'Wrong Buildx architecture')
    require(re.fullmatch('[0-9a-f]{64}', data['binarySha256']) is not None
            and data['binarySha256'] == binding['binarySha256'], 'Actual binary SHA differs from built source receipt')
    packages = data['packages'].splitlines()
    require(packages and len(packages) == len(set(packages)), 'Incomplete/duplicate compiled package inventory')
    require(MAIN in packages, 'Buildx main package was not compiled')
    require([p for p in packages if p == 'github.com/docker/docker' or p.startswith('github.com/docker/docker/')] == [NAMES],
            'Legacy Docker contains more than namesgenerator; daemon absence is unproven')
    require('github.com/moby/go-archive/compression' in packages
            and 'github.com/moby/go-archive' not in packages, 'Unexpected archive extractor compilation')
    source_files = {}
    for line in data['files'].splitlines():
        package, path, digest = line.split('\t')
        require(package in packages and path.startswith(('source/', 'module/', 'goroot/'))
                and not any(p in ['', '.', '..'] for p in path.split('/'))
                and '\\' not in path and re.fullmatch('[0-9a-f]{64}', digest), 'Unsafe/invalid compiled file inventory')
        require((package, path) not in source_files, 'Duplicate compiled file')
        source_files[package, path] = digest
    require({k: v for k, v in source_files.items() if k[0] == NAMES} == {(NAMES, NAME_FILE): NAME_SHA},
            'Namesgenerator source bytes do not match the official module')
    before, after = data['modulesBefore'].splitlines(), data['modulesAfter'].splitlines()
    changed = ('golang.org/x/net ', 'github.com/moby/go-archive ')
    require([s for s in before if not s.startswith(changed)] == [s for s in after if not s.startswith(changed)],
            'Unrelated module version changed')
    for module, old, new in [('golang.org/x/net', 'v0.59.0', 'v0.60.0'), ('github.com/moby/go-archive', 'v0.2.0', 'v0.3.0')]:
        require([s for s in before if s.startswith(module + ' ')] == [module + ' ' + old]
                and [s for s in after if s.startswith(module + ' ')] == [module + ' ' + new], 'Wrong fixed module version')
    info = data['buildInfo'].splitlines()
    require(info and info[0].endswith(': go1.27.3'), 'Actual binary has a different compiler')
    deps = {}
    settings = {}
    for line in info[1:]:
        fields = line.strip().split('\t')
        if fields[0] == 'dep':
            require(fields[1] not in deps, 'Duplicate binary module'); deps[fields[1]] = fields[2]
        if fields[0] == 'build':
            key, value = fields[1].split('=', 1)
            require(key not in settings, 'Duplicate binary setting'); settings[key] = value
    require(all(settings.get(k) == v for k, v in {'GOOS': 'linux', 'GOARCH': binding['goarch'], 'CGO_ENABLED': '0'}.items()),
            'Actual binary platform differs from source check')
    require(all(deps.get(k) == v for k, v in {'golang.org/x/net': 'v0.60.0', 'github.com/moby/go-archive': 'v0.3.0',
                                            'github.com/docker/docker': 'v28.5.2+incompatible'}.items()), 'Actual binary module identity changed')
    require(f"github.com/docker/buildx {PINS['downstreamVersion']} {PINS['sourceCommit']}" in data['version'], 'Wrong downstream tool identity')
    require(set(data['osvInputs']) == IDS and all(sha(data['osvInputs'][k]) == v for k, v in OSV_PINS.items()),
            'Canonical four vulnerability input bytes do not match their official pins')
    checks = list(objects(data['sourceCheck']))
    configs = [x['config'] for x in checks if 'config' in x]
    require(len(configs) == 1 and configs[0]['scanner_name'] == 'govulncheck'
            and configs[0]['scanner_version'] == 'v1.9.0' and configs[0]['scan_mode'] == 'source'
            and configs[0]['scan_level'] == 'package', 'Native package source check missing or replaced by stripped-binary fallback')
    sboms = [x['SBOM'] for x in checks if 'SBOM' in x]
    require(len(sboms) == 1 and sboms[0]['go_version'] == 'go1.27.3' and sboms[0]['roots'] == [MAIN],
            'Native source check did not inspect this Buildx program')
    scanned = unique_pairs((m['path'], m.get('version', '')) for m in sboms[0]['modules'])
    require(all(scanned.get(k) == v for k, v in deps.items()), 'Source check module versions differ from the binary')
    emitted = {x['osv']['id'] for x in checks if 'osv' in x}
    require({'GO-2026-5617', 'GO-2026-5746'} <= emitted <= IDS,
            'Daemon source checks missing or an unbounded OSV was emitted')
    for item in checks:
        if 'finding' in item:
            finding = item['finding']
            require(finding['osv'] in IDS, 'Unbounded source-check result')
            require(not any(frame.get('package') or frame.get('function') for frame in finding['trace']),
                    'Vulnerable package or function is present in the source check')
    raw = load(data['raw'])
    scoped = [(target, f) for target, f in findings(raw) if f.get('PkgIdentifier', {}).get('PURL') == PURL]
    require(scoped and all(target == TARGET for target, _ in scoped), 'The same legacy PURL occurs in another binary')
    require({f['VulnerabilityID'] for _, f in scoped if f['VulnerabilityID'] in CVES} == CVES,
            'The two raw daemon controls are missing')
    require({p for p in vex_pairs(data['vex']) if p[1] == PURL} == {(cve, PURL) for cve in CVES},
            'Daemon VEX exceeds the two qualified scopes')
    return {'schemaVersion': 1, 'status': 'BUILDX_DAEMON_CODE_ABSENCE_VERIFIED', 'target': TARGET,
            'purl': PURL, 'sourceCommit': binding['sourceCommit'], 'sourceArchiveSha256': binding['sourceArchiveSha256'],
            'binarySha256': binding['binarySha256'], 'rawSha256': sha(data['raw']),
            'vexSha256': sha(json.dumps(data['vex'], sort_keys=True)),
            'compiledPackageCount': len(packages), 'compiledSourceFileCount': len(source_files),
            'packagesSha256': sha(data['packages']), 'sourceFilesSha256': sha(data['files']),
            'modulesBeforeSha256': sha(data['modulesBefore']), 'modulesAfterSha256': sha(data['modulesAfter']),
            'canonicalOsvSha256': dict(OSV_PINS), 'emittedOsv': sorted(emitted),
            'sourceCheckSha256': sha(data['sourceCheck']), 'scope': sorted(CVES)}


def native_check(data):
    require(data['guard']['status'] == 'BUILDX_DAEMON_CODE_ABSENCE_VERIFIED'
            and sha(data['raw']) == data['guard']['rawSha256']
            and sha(json.dumps(data['vex'], sort_keys=True)) == data['guard']['vexSha256'],
            'Raw report or VEX changed after the source guard')
    raw, effective = load(data['raw']), load(data['effective'])
    allowed = vex_pairs(data['vex'])
    canonical = lambda target, finding: json.dumps([target, finding], sort_keys=True, separators=(',', ':'))
    kept = Counter(canonical(t, f) for t, f in findings(raw) if pair(f) not in allowed)
    actual = Counter(canonical(t, f) for t, f in findings(effective))
    require(kept == actual, 'Native VEX changed an unrelated finding or retained a qualified scope')
    return {'schemaVersion': 1, 'status': 'NATIVE_DAPPER_VEX_VERIFIED', 'rawSha256': sha(data['raw']),
            'effectiveSha256': sha(data['effective']), 'retainedFindings': sum(actual.values()),
            'rawFindings': sum(1 for _ in findings(raw)), 'rawRetainedUnchanged': True}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--native', action='store_true')
    args = parser.parse_args()
    try:
        content = sys.stdin.buffer.read(64 * 1024 * 1024 + 1)
        require(len(content) <= 64 * 1024 * 1024, 'Evidence exceeds size limit')
        data = load(content.decode('utf-8'))
        print(json.dumps(native_check(data) if args.native else source_check(data), sort_keys=True))
    except (ValueError, KeyError, TypeError, UnicodeError) as error:
        print('BUILDX_CODE_ABSENCE_FAILED: ' + str(error), file=sys.stderr)
        sys.exit(1)
