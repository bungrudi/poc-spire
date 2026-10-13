#!/usr/bin/env python3
"""Run the full approved owned baseline in dependency order, with checked failure cleanup."""
import argparse
import importlib
import json
from pathlib import Path

import kubernetes_demo as runtime

STAGES = ('kubernetes_network', 'kubernetes_budget', 'kubernetes_kubelet', 'kubernetes_mesh',
          'kubernetes_build', 'kubernetes_images', 'kubernetes_spire', 'kubernetes_policy',
          'kubernetes_application', 'kubernetes_finish_bootstrap')


def start():
    value = None
    stage = 'preflight'
    revision = runtime.execute(['git','rev-parse','HEAD']).strip()
    try:
        runtime.start()
        value = runtime.descriptor()
        for stage in STAGES:
            print('Starting owned stage: ' + stage, flush=True)
            module = importlib.import_module(stage)
            module.main('normal') if stage == 'kubernetes_images' else module.main()
    except Exception:
        if value is not None:
            from kubernetes_diagnostics import capture
            try: capture(value, stage)
            except Exception: pass  # Diagnostic failure never prevents owned cleanup.
            runtime.private_json(runtime.RUNTIME / 'diagnostics' / ('failed-'+value['run']+'.json'),
                                 {'run':value['run'],'source_revision':revision,'stage':stage,
                                  'status':'development startup failed; no acceptance pass'})
            if runtime.descriptor() != value:
                raise RuntimeError('failed run ownership changed; refuse cleanup of another run') from None
            runtime.stop()
        raise


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=('start','stop'))
    args=parser.parse_args()
    start() if args.action=='start' else runtime.stop()
