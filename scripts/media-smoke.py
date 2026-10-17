#!/usr/bin/env python3
"""Exercise derived-media jobs against a disposable smoke library."""
import json
import sys
import time
import urllib.request

base, host = sys.argv[1:3]

def request(path, body=None, headers=None):
    req = urllib.request.Request(base + '/api/v1' + path,
        data=None if body is None else json.dumps(body).encode(),
        headers={'Host': host, 'Content-Type': 'application/json', **(headers or {})})
    return urllib.request.urlopen(req, timeout=15)

with request('/files') as response:
    files = json.load(response)['files']
for file in files:
    if file['name'] not in ('sample.png', 'tiled.heic', 'preview.mp4', 'preview.mov', 'two-pages.pdf'):
        continue
    if file['name'] == 'preview.mov':
        assert file['kind'] == 'video' and file['media_type'] == 'video/quicktime', file
    if file['name'] == 'two-pages.pdf':
        assert file['media_type'] == 'application/pdf', file
    recipes = ['video_thumb_512_v2' if file['name'].endswith(('.mp4', '.mov')) else 'thumb_512_v1', 'preview_1080p_v1' if file['name'].endswith(('.mp4', '.mov')) else 'preview_2048_v1']
    for recipe in recipes:
        path = '/files/' + file['id'] + '/derived/' + recipe
        with request(path, {}) as response:
            state = json.load(response)
        deadline = time.monotonic() + 120
        while state['state'] in ('queued', 'running') and time.monotonic() < deadline:
            time.sleep(0.25)
            with request(path) as response:
                state = json.load(response)
        assert state['state'] == 'ready', state
        if file['name'] == 'two-pages.pdf':
            expected = (256, 512) if recipe == 'thumb_512_v1' else (1024, 2048)
            assert (state['result']['width'], state['result']['height']) == expected, state
        if file['name'] == 'tiled.heic' and recipe == 'preview_2048_v1':
            assert (state['result']['width'], state['result']['height']) == (2048, 1365), state
        with request(path + '/content', headers={'Range': 'bytes=0-7'}) as response:
            assert response.status == 206
            assert len(response.read()) == 8
            assert response.headers['Content-Type'] == ('video/mp4' if recipe == 'preview_1080p_v1' else 'image/jpeg')
        print(file['name'], recipe, state['result']['width'], state['result']['height'], flush=True)
assert {'sample.png', 'tiled.heic', 'preview.mp4', 'preview.mov', 'two-pages.pdf'} <= {file['name'] for file in files}
