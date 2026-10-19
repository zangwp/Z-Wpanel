const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const {test} = require('node:test');

const ok = data => ({success:true, data});
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => {resolve=yes; reject=no;}); return {promise, resolve, reject}; };
const tick = () => new Promise(resolve => setImmediate(resolve));
function setup(file, factory, respond) {
    const html = fs.readFileSync(path.join(__dirname, '../../web/templates', file), 'utf8');
    const source = [...html.matchAll(/<script[^>]*>([\s\S]*?)<\/script>/g)].map(match => match[1].replace(/{{[\s\S]*?}}/g, 'translated')).join('\n');
    const calls=[], timers=new Map(); let nextTimer=1;
    const context = {URL, URLSearchParams, AbortController, TextEncoder, t:key=>key, currentLocale:()=> 'en-US',
        showToast(){}, confirmModal:async()=>true, deferTask(){},
        setTimeout(fn){const id=nextTimer++;timers.set(id,fn);return id;}, clearTimeout(id){timers.delete(id);}, setInterval:()=>1, clearInterval(){},
        window:{location:{pathname:'/panel/websites/1',hash:'',search:''},history:{replaceState(){}}},
        document:{hidden:false,body:{dataset:{panelPrefix:'/panel'}},querySelector:()=>null},
        api:async(url,options={})=>{calls.push({url,options});return respond(url,options);}};
    vm.createContext(context); vm.runInContext(source,context);
    return {page:context[factory](1),context,calls,timers,html};
}
const savedSettings=()=>({username:'admin',basic_auth_user:'gateway',panel_title:'Saved title',hostname:'saved-host',timezone:'UTC',
    server_time:Date.now(),ntp_synced:true,ntp_enabled:true,ntp_service:'timesyncd',panel_auto_update:{panel_auto_update_release_delay_minutes:'15'}});
const savedSite=()=>({id:1,domain:'site.example',site_type:'wordpress',status:'active',fastcgi_cache_ttl:300,log_retention_days:14,ssl_export_enabled:false});

test('NTP only refreshes observed time and keeps all other forms enabled and unsaved',async()=>{
    const read=deferred(); let initial=true;
    const f=setup('settings.html','panelSettings',async(_url,options)=>options.method?ok({}):initial?ok(savedSettings()):read.promise);
    await f.page.fetchSettings();initial=false;
    f.page.panelTitle='draft-title';f.page.hostname='draft-host';f.page.autoUpdate.delay=90;
    const pending=f.page.syncNTP();await tick();
    assert.equal(f.page.accountLoaded,true);assert.equal(f.page.ntpSyncing,true);
    read.resolve(ok(savedSettings()));await pending;
    assert.equal(f.page.panelTitle,'draft-title');assert.equal(f.page.hostname,'draft-host');assert.equal(f.page.autoUpdate.delay,90);
    assert.equal(f.page.ntpSyncing,false);assert.equal(f.timers.size,0);
});

test('late NTP refresh after page destruction is discarded and scheduled refresh is cancelled',async()=>{
    const read=deferred();const f=setup('settings.html','panelSettings',()=>read.promise);
    f.page.accountLoaded=true;f.page.ntpSynced=false;f.page.ntpRefreshTimer=f.context.setTimeout(()=>{},5000);
    const pending=f.page.refreshTimeStatus();f.page.destroy();
    read.resolve(ok(savedSettings()));await pending;
    assert.equal(f.page.ntpSynced,false);assert.equal(f.timers.size,0);assert.equal(f.page.timeStatusController,null);
});

test('settings refresh does not erase drafts and an older response cannot replace a newer read',async()=>{
    const f=setup('settings.html','panelSettings',async()=>ok(savedSettings()));
    await f.page.fetchSettings();f.page.hostname='unsaved-host';
    const one=deferred(),two=deferred();let reads=0;f.context.api=()=>++reads===1?one.promise:two.promise;
    const old=f.page.fetchSettings(),newer=f.page.fetchSettings();
    two.resolve(ok({...savedSettings(),panel_title:'Newest title'}));await newer;
    one.resolve(ok({...savedSettings(),panel_title:'Old title'}));await old;
    assert.equal(f.page.hostname,'unsaved-host');assert.equal(f.page.panelTitle,'Newest title');
});

test('saving SSL export retains cache, PHP, logging and later drafts without replacing Nginx features',async()=>{
    const read=deferred(),f=setup('website_detail.html','websiteDetail',async(_url,options)=>options.method?ok({}):read.promise);
    f.page.site=savedSite();Object.assign(f.page,f.page.detailFormValues(f.page.site));
    for(const name of ['loadSiteJobs','loadSSLRenewal','loadMaintenance','fetchSiteSecurityStatus','fetchNginxCustom','fetchPluginStatus','fetchCDNGroups','fetchAIDevelopment'])f.page[name]=async()=>{};
    f.page.fcacheTTL=600;f.page.wpMemoryLimit='256M';f.page.logRetentionDays=30;f.page.sslExportEnabled=true;
    const save=f.page.saveSSLExport();await tick();f.page.monitoringInterval=10;
    read.resolve(ok({...savedSite(),ssl_export_enabled:true}));await save;
    assert.equal(f.page.fcacheTTL,600);assert.equal(f.page.wpMemoryLimit,'256M');assert.equal(f.page.logRetentionDays,30);assert.equal(f.page.monitoringInterval,10);
    assert.equal(f.calls.length,2);assert(f.html.includes('saveNginxCustom()'));assert(f.html.includes('clearFCache()'));
});

test('database sync does not reload backup settings or overwrite prefix typing',async()=>{
    const read=deferred(),f=setup('database_detail.html','databaseDetail',async(_url,options)=>options.method?ok({}):read.promise);
    f.page.site={id:1,table_prefix:'wp_'};f.page.loading=false;f.page.editTablePrefix='wp_';f.page.backupEnabled=false;f.page.backupKeepCount=30;
    const pending=f.page.fixWPConfig();await tick();f.page.editTablePrefix='typed_after_submit_';
    assert.equal(f.page.loading,false);assert.equal(f.page.dbAction,'sync');
    read.resolve(ok({id:1,table_prefix:'wp_'}));await pending;
    assert.equal(f.page.editTablePrefix,'typed_after_submit_');assert.equal(f.page.backupEnabled,false);assert.equal(f.page.backupKeepCount,30);
    assert.deepEqual(f.calls.map(call=>call.url),['/websites/1/fix-wp-config','/websites/1']);
    assert.equal(f.page.dbSubmitting,false);assert.equal(f.page.dbAction,'');
});

test('Cloudflare status refresh retains binding drafts and does not allow syncing unsaved settings',async()=>{
    const data={site_id:1,hostname:'site.example',scope:'shared_web_bans',enabled:false,account_id:'a'.repeat(32),token_configured:true,zone_id:'',rule_id:'',active_ip_count_known:true,active_ip_count:0,sync_status:'disabled'};
    const f=setup('site_cloudflare_security.html','siteCloudflareSecurity',async()=>ok(data));f.page.site=savedSite();f.page.applyCloudflareConfig(data,1);
    f.page.cfForm={enabled:true,account_id:'b'.repeat(32)};f.page.cfToken='unsaved-token';await f.page.loadCloudflare();
    assert.equal(f.page.cfForm.enabled,true);assert.equal(f.page.cfForm.account_id,'b'.repeat(32));assert.equal(f.page.cfToken,'unsaved-token');assert.equal(f.page.canSyncCloudflare(),false);
});

test('anomaly check updates evidence while retaining configuration drafts and one resource lock',async()=>{
    const read=deferred(),f=setup('wp_anomaly_panel.html','wpAnomalyMonitor',()=>read.promise);
    f.page.state={enabled:true,threshold:5};f.page.enabled=false;f.page.threshold=55;
    const pending=f.page.check();await f.page.save();await f.page.check();
    assert.equal(f.calls.length,1);assert.equal(f.page.action,'check');
    read.resolve(ok({enabled:true,threshold:5,last_success:123}));await pending;
    assert.equal(f.page.enabled,false);assert.equal(f.page.threshold,55);assert.equal(f.page.state.last_success,123);assert.equal(f.page.busy,false);
});

test('only the selected DNS preset/action shows progress and confirmation owns its lock',async()=>{
    const confirm=deferred(),write=deferred(),f=setup('vps.html','vpsPage',()=>write.promise);
    f.context.confirmModal=()=>confirm.promise;f.page.overview={dns:{configurable:true}};
    const pending=f.page.applyDNS('cloudflare');await f.page.applyDNS('google');await f.page.testDNS('google');
    assert.equal(f.page.dnsAction,'apply');assert.equal(f.page.dnsPreset,'cloudflare');assert.equal(f.calls.length,0);
    confirm.resolve(true);await tick();assert.equal(f.calls.length,1);assert.equal(f.calls[0].options.body.preset,'cloudflare');
    write.resolve(ok({configurable:true}));await pending;assert.equal(f.page.dnsAction,'');assert.equal(f.page.dnsBusy,false);
    assert(f.html.includes("dnsAction === 'test' && dnsPreset === preset.id"));assert(f.html.includes("dnsAction === 'apply' && dnsPreset === preset.id"));
});

test('white-list updates use a bounded independent task without disabling security settings',async()=>{
    const read=deferred(),f=setup('security.html','securitySettings',async(url,options)=>options.method?ok({task_id:'task1',kind:'refresh_whitelist',status:'waiting'}):read.promise);
    f.page.settingsLoaded=true;f.page.settings={whitelist_ips:'unsaved'};
    const pending=f.page.refreshWhitelist();await tick();
    assert.equal(f.page.settingsDisabled(),false);assert.equal(f.page.settingsSaving,false);assert.equal(f.page.whitelistRefreshing,true);
    await f.page.refreshWhitelist();assert.equal(f.calls.filter(call=>call.options.method==='POST').length,1);
    read.resolve(ok({task_id:'task1',kind:'refresh_whitelist',status:'running'}));await pending;
    assert.equal(f.timers.size,1);assert.equal(f.page.whitelistNotice,'security.refresh_running');
    f.page.destroy();assert.equal(f.timers.size,0);assert.equal(f.page.whitelistRefreshing,false);
});

test('an invalid whitelist task response cannot invent successful completion or start polling',async()=>{
    const f=setup('security.html','securitySettings',async()=>ok({}));f.page.settingsLoaded=true;
    await f.page.refreshWhitelist();assert.equal(f.page.whitelistRefreshing,false);assert.equal(f.page.whitelistError,'security.refresh_status_unavailable');assert.equal(f.timers.size,0);
});
