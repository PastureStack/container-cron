#!/usr/bin/env python3
"""Focused source-bound applicability and native VEX regression controls."""
import copy
import json
import unittest
from unittest.mock import patch

import check_buildx_code_absence as guard


def finding(cve, purl=guard.PURL):
    return {'VulnerabilityID': cve, 'PkgIdentifier': {'PURL': purl}, 'Severity': 'HIGH'}


def fixture():
    binding = dict(guard.PINS, goarch='amd64', binarySha256='a' * 64)
    modules = {'github.com/docker/docker': 'v28.5.2+incompatible',
               'github.com/moby/go-archive': 'v0.3.0', 'golang.org/x/net': 'v0.60.0'}
    before = dict(modules, **{'github.com/moby/go-archive': 'v0.2.0', 'golang.org/x/net': 'v0.59.0'})
    checks = [{'config': {'scanner_name': 'govulncheck', 'scanner_version': 'v1.9.0',
                          'scan_mode': 'source', 'scan_level': 'package'}},
              {'SBOM': {'go_version': 'go1.27.3', 'roots': [guard.MAIN],
                        'modules': [{'path': k, 'version': v} for k, v in modules.items()]}}]
    checks.extend({'osv': {'id': identifier}} for identifier in sorted(guard.IDS))
    # Legacy module presence is expected; imported/called daemon code is not.
    checks.append({'finding': {'osv': 'GO-2026-5746', 'trace': [
        {'module': 'github.com/docker/docker', 'version': 'v28.5.2+incompatible'}]}})
    raw = {'Results': [{'Target': guard.TARGET, 'Vulnerabilities': [
        finding(cve) for cve in sorted(guard.CVES)]},
        {'Target': 'usr/bin/other', 'Vulnerabilities': [finding('CVE-OTHER', 'pkg:generic/other@1')]}]}
    vex = {'statements': [{'vulnerability': {'name': cve}, 'products': [{'@id': guard.PURL}],
                           'status': 'not_affected', 'justification': 'vulnerable_code_not_present'}
                          for cve in sorted(guard.CVES)]}
    return {
        'binding': ''.join(f'{k}\t{v}\n' for k, v in binding.items()),
        'binarySha256': binding['binarySha256'],
        'packages': '\n'.join([guard.MAIN, guard.NAMES, 'github.com/moby/go-archive/compression']) + '\n',
        'files': f'{guard.NAMES}\t{guard.NAME_FILE}\t{guard.NAME_SHA}\n',
        'modulesBefore': ''.join(f'{k} {v}\n' for k, v in before.items()),
        'modulesAfter': ''.join(f'{k} {v}\n' for k, v in modules.items()),
        'buildInfo': 'docker-buildx: go1.27.3\n' + ''.join(f'\tdep\t{k}\t{v}\th1:example\n' for k, v in modules.items())
                     + '\tbuild\tGOOS=linux\n\tbuild\tGOARCH=amd64\n\tbuild\tCGO_ENABLED=0\n',
        'version': f"github.com/docker/buildx {guard.PINS['downstreamVersion']} {guard.PINS['sourceCommit']}\n",
        'sourceCheck': '\n'.join(json.dumps(c) for c in checks),
        'osvInputs': {identifier: json.dumps({'id': identifier}) for identifier in guard.IDS},
        'raw': json.dumps(raw), 'vex': vex,
    }


class BuildxEvidenceTests(unittest.TestCase):
    def setUp(self):
        # Small synthetic OSVs keep the tests independent of network/local files.
        # Production pins are fixed constants; the CLI has no pin override.
        hashes = {k: guard.sha(v) for k, v in fixture()['osvInputs'].items()}
        pins = patch.dict(guard.OSV_PINS, hashes, clear=True)
        pins.start(); self.addCleanup(pins.stop)

    def test_exact_source_and_module_only_notice_pass(self):
        result = guard.source_check(fixture())
        self.assertEqual(result['status'], 'BUILDX_DAEMON_CODE_ABSENCE_VERIFIED')
        self.assertEqual(result['scope'], sorted(guard.CVES))
        # Fixed module reports may be absent from scanner output; their exact
        # canonical input bytes are independently required and SHA checked.
        data = fixture()
        data['sourceCheck'] = '\n'.join(json.dumps(x) for x in guard.objects(data['sourceCheck'])
                                      if x.get('osv', {}).get('id') not in {'GO-2026-6253', 'GO-2026-6611'})
        guard.source_check(data)

    def test_wrong_source_and_binary_identity_rejected(self):
        for field, old in [('binding', guard.PINS['sourceCommit']),
                           ('binding', guard.PINS['sourceArchiveSha256']),
                           ('binding', guard.PINS['updatedGoModSha256']),
                           ('binarySha256', 'a' * 64), ('buildInfo', 'go1.27.3'),
                           ('buildInfo', 'GOARCH=amd64'), ('buildInfo', 'v0.60.0')]:
            with self.subTest(field=field, old=old):
                data = fixture(); data[field] = data[field].replace(old, 'wrong', 1)
                with self.assertRaises(ValueError):
                    guard.source_check(data)

    def test_full_daemon_and_wrong_namesgenerator_source_rejected(self):
        for field, value in [('packages', 'github.com/docker/docker/daemon\n'),
                             ('packages', 'github.com/moby/go-archive\n'),
                             ('files', f'{guard.NAMES}\tmodule/extra.go\t' + 'b' * 64 + '\n')]:
            with self.subTest(value=value):
                data = fixture(); data[field] += value
                with self.assertRaises(ValueError):
                    guard.source_check(data)
        data = fixture(); data['files'] = data['files'].replace(guard.NAME_SHA, 'b' * 64)
        with self.assertRaises(ValueError):
            guard.source_check(data)

    def test_other_binary_same_purl_rejected(self):
        for cve in ['CVE-2026-41567', 'CVE-UNREVIEWED']:
            data = fixture(); raw = json.loads(data['raw'])
            raw['Results'].append({'Target': 'usr/bin/daemon', 'Vulnerabilities': [finding(cve)]})
            data['raw'] = json.dumps(raw)
            with self.subTest(cve=cve), self.assertRaises(ValueError):
                guard.source_check(data)

    def test_unrelated_module_bump_rejected(self):
        data = fixture(); data['modulesAfter'] += 'example.org/unrelated v2.0.0\n'
        with self.assertRaises(ValueError):
            guard.source_check(data)

    def test_incomplete_or_other_program_source_scan_rejected(self):
        for old, new in [('"source"', '"binary"'), ('"package"', '"module"'),
                         ('"SBOM"', '"sbom"'),
                         (guard.MAIN, 'example.org/other'), ('go1.27.3', 'go1.26.10'),
                         ('GO-2026-6253', 'GO-MISSING')]:
            data = fixture(); data['sourceCheck'] = data['sourceCheck'].replace(old, new)
            with self.subTest(old=old), self.assertRaises(ValueError):
                guard.source_check(data)
        for tamper in ['missing-input', 'changed-input', 'missing-daemon-output']:
            data = fixture()
            if tamper == 'missing-input': data['osvInputs'].pop('GO-2026-6253')
            if tamper == 'changed-input': data['osvInputs']['GO-2026-6611'] += '\n'
            if tamper == 'missing-daemon-output':
                data['sourceCheck'] = '\n'.join(json.dumps(x) for x in guard.objects(data['sourceCheck'])
                                              if x.get('osv', {}).get('id') != 'GO-2026-5746')
            with self.subTest(tamper=tamper), self.assertRaises(ValueError):
                guard.source_check(data)

    def test_imported_or_called_vulnerable_code_rejected(self):
        for frame in [{'module': 'github.com/docker/docker', 'package': 'github.com/docker/docker/daemon'},
                      {'module': 'github.com/docker/docker', 'function': 'Daemon.openContainerFS'}]:
            data = fixture(); data['sourceCheck'] += '\n' + json.dumps(
                {'finding': {'osv': 'GO-2026-5617', 'trace': [frame]}})
            with self.subTest(frame=frame), self.assertRaises(ValueError):
                guard.source_check(data)

    def test_vex_cannot_extend_legacy_module_scope(self):
        data = fixture(); statement = copy.deepcopy(data['vex']['statements'][0])
        statement['vulnerability']['name'] = 'CVE-UNREVIEWED'
        data['vex']['statements'].append(statement)
        with self.assertRaises(ValueError):
            guard.source_check(data)

    def test_native_vex_keeps_complete_unrelated_finding(self):
        data = fixture(); data['guard'] = guard.source_check(data)
        effective = json.loads(data['raw']); effective['Results'][0]['Vulnerabilities'] = []
        data['effective'] = json.dumps(effective)
        self.assertTrue(guard.native_check(data)['rawRetainedUnchanged'])
        self.assertEqual(guard.native_check(data)['retainedFindings'], 1)

    def test_native_tamper_or_unrelated_removal_rejected(self):
        for tamper in ['raw', 'vex', 'remove', 'alter', 'retain']:
            data = fixture(); data['guard'] = guard.source_check(data)
            effective = json.loads(data['raw']); effective['Results'][0]['Vulnerabilities'] = []
            if tamper == 'raw': data['raw'] += '\n'
            if tamper == 'vex': data['vex']['statements'].pop()
            if tamper == 'remove': effective['Results'][1]['Vulnerabilities'] = []
            if tamper == 'alter': effective['Results'][1]['Vulnerabilities'][0]['Severity'] = 'LOW'
            if tamper == 'retain': effective = json.loads(data['raw'])
            data['effective'] = json.dumps(effective)
            with self.subTest(tamper=tamper), self.assertRaises(ValueError):
                guard.native_check(data)


if __name__ == '__main__':
    unittest.main()
