const fs = require('fs'), vm = require('vm'), assert = require('assert'), path = require('path');
process.chdir(path.resolve(__dirname, '../..'));
const source = file => fs.readFileSync('web/templates/' + file + '.html', 'utf8');
const script = file => [...source(file).matchAll(/<script[^>]*>([\s\S]*?)<\/script>/g)].map(m => m[1].replace(/{{[\s\S]*?}}/g, 'null')).join('\n');
const listeners = new Map();
const context = {
    t: key => key, showToast() {}, clearInterval() {},
    document: { getElementById: () => null },
    window: { location: { hash: '#security-maintenance', pathname: '/panel/websites/7', search: '' },
        history: { replaceState(_state, _unused, url) { context.window.location.hash = '#' + url.split('#')[1]; } },
        addEventListener: (name, fn) => listeners.set(name, fn),
        removeEventListener: name => listeners.delete(name) },
    api: async () => { throw Error('offline'); }
};
vm.createContext(context);
vm.runInContext(script('website_detail'), context);
vm.runInContext(script('dashboard'), context);
(async () => {
    const page = context.websiteDetail();
    let fetched = 0, logs = 0;
    page.fetchDetail = () => fetched++;
    page.fetchLogs = () => logs++;
    page.init();
    assert.equal(page.detailTab, 'security');
    assert.equal(fetched, 1);
    assert.equal(logs, 0);
    page.site = { id: 7, site_type: 'wordpress' };
    page.setDetailTab('logs');
    assert.equal(logs, 1);
    assert.equal(context.window.location.hash, '#logs');
    context.window.location.hash = '#site-performance';
    listeners.get('hashchange')();
    assert.equal(page.detailTab, 'cache');
    page.site.site_type = 'php';
    page.setDetailTab('security');
    assert.equal(page.detailTab, 'overview');
    page.destroy();
    assert.equal(listeners.size, 0);
    const version = context.versionCheck();
    await version.check();
    assert.equal(version.checking, false);
    assert.equal(version.checkError, 'offline');
    assert(!version.checked);
    context.api = async url => ({success: true, data: url === '/update/check' ? {has_update: false} : {count: 0}});
    version.hasUpdate = true;
    await version.check();
    assert.equal(version.hasUpdate, false);
    assert.equal(version.checkError, '');
    assert(version.checked);
    assert(!source('base').includes('href="/{{$.RandomSuffix}}/wordpress-overview"'));
    assert(source('website_navigation').includes('/wordpress-overview'));
    for (const feature of ['fetchNginxCustom()', 'fcacheEnabled', 'companion-plugin-controls']) assert(source('website_detail').includes(feature));
    assert(!source('website_detail').includes('/php-runtime'));
    assert(!source('firewall').includes('olswpanel'));
    assert(source('firewall').includes('value="yubwpanel-login"'));
    console.log('Shared tabs/deep links, cleanup, visible update failures, navigation and Nginx controls passed');
})().catch(error => { console.error(error); process.exit(1); });
