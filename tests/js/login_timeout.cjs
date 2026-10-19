const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const {test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../../web/templates/login.html'),'utf8');
const script=[...html.matchAll(/<script[^>]*>([\s\S]*?)<\/script>/g)].find(match=>match[1].includes('function loginForm'))[1];
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function setup(){const timers=new Map(),calls=[],token={content:'old'};let next=1;
    const context={AbortController,t:key=>key,setTimeout(fn,delay){const id=next++;timers.set(id,{fn,delay});return id;},clearTimeout(id){timers.delete(id);},
        document:{body:{dataset:{panelPrefix:'/panel'}},querySelector:()=>token},window:{location:{href:''}},
        fetch:async()=>({ok:true,json:async()=>({success:true,data:{token:'fresh'}})}),
        api:async(url,options)=>{calls.push({url,options});return {success:false,message:'invalid credentials'};}};
    vm.createContext(context);vm.runInContext(script,context);const page=context.loginForm();page.username='submitted-user';page.password='submitted-password';page.code=' 123456 ';
    return {page,context,timers,calls,token};
}

test('stalled CSRF refresh aborts after 20 seconds and restores the login form with a retry message',async()=>{
    const f=setup();let signal;
    f.context.fetch=(_url,options)=>new Promise((_resolve,reject)=>{signal=options.signal;signal.addEventListener('abort',()=>reject(new Error('aborted')),{once:true});});
    const pending=f.page.login();await tick();assert.equal(f.page.loading,true);assert.equal(f.timers.size,1);
    const timer=[...f.timers.values()][0];assert.equal(timer.delay,20000);timer.fn();await pending;
    assert.equal(signal.aborted,true);assert.equal(f.page.loading,false);assert.equal(f.page.error,'auth.login_timeout');assert.equal(f.timers.size,0);assert.equal(f.calls.length,0);
});

test('the login POST is bounded and uses one immutable credentials snapshot through a CSRF retry',async()=>{
    const f=setup();let release;f.context.fetch=()=>new Promise(resolve=>{release=resolve;});
    let writes=0;f.context.api=async(url,options)=>{f.calls.push({url,options});if(++writes===1){const error=new Error('csrf mismatch');error.status=403;throw error;}return {success:false,message:'invalid credentials'};};
    const pending=f.page.login();f.page.username='new-draft';f.page.password='new-draft-password';f.page.code='654321';await f.page.login();
    release({ok:true,json:async()=>({success:true,data:{token:'fresh'}})});await tick();
    release({ok:true,json:async()=>({success:true,data:{token:'fresh'}})});await pending;
    assert.equal(f.calls.length,2);for(const call of f.calls){assert.equal(call.options.timeout,20000);assert.equal(call.options.body.username,'submitted-user');assert.equal(call.options.body.password,'submitted-password');assert.equal(call.options.body.code,'123456');}
    assert.equal(f.page.password,'new-draft-password');assert.equal(f.page.loading,false);assert.equal(f.timers.size,0);
});

test('a POST timeout gives a retry message without incorrectly reporting wrong credentials',async()=>{
    const f=setup();f.context.api=async()=>{const error=new Error('request timeout');error.code='request_timeout';throw error;};await f.page.login();
    assert.equal(f.page.error,'auth.login_timeout');assert.equal(f.page.loading,false);assert.equal(f.page.password,'submitted-password');assert.equal(f.timers.size,0);
});

test('a completed Basic Auth challenge is distinguished from the panel account password',async()=>{
    const f=setup();f.context.fetch=async()=>({status:401,ok:false,headers:{get:()=> 'Basic realm="YUB WPanel Authentication"'},json:async()=>{throw Error('should not parse empty challenge');}});
    await f.page.login();assert.equal(f.page.error,'auth.gateway_auth_required');assert.equal(f.page.loading,false);assert.equal(f.calls.length,0);assert.equal(f.timers.size,0);
});
