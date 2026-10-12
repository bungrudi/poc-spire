// Independent JOSE codec fixture. Private keys remain non-extractable in this
// process; the test exchanges public certificates and command wire bytes by pipe.
import {createInterface} from 'node:readline';
import {CompactSign, compactVerify, decodeProtectedHeader, generateKeyPair, exportSPKI, importX509, importJWK} from 'jose';
const pair=await generateKeyPair('ES256',{extractable:false});
const send=value=>process.stdout.write(JSON.stringify(value)+'\n');
send({spki:await exportSPKI(pair.publicKey),version:'6.1.0'});
const lines=createInterface({input:process.stdin,crlfDelay:Infinity});
for await(const line of lines){
 try{
  if(line.length>65536)throw new Error('input_bounds');
  const input=JSON.parse(line);
  if(input.kind==='verify'){
   const header=decodeProtectedHeader(input.compact);
   const der=header.x5c[0];
   const pem='-----BEGIN CERTIFICATE-----\n'+der.match(/.{1,64}/g).join('\n')+'\n-----END CERTIFICATE-----';
   const publicKey=await importX509(pem,'ES256');
   const good=await compactVerify(input.compact,publicKey,{algorithms:['ES256']});
   let alteredRejected=false,curveRejected=false;
   const parts=input.compact.split('.');const payload=Buffer.from(parts[1],'base64url');payload[0]^=1;parts[1]=payload.toString('base64url');
   try{await compactVerify(parts.join('.'),publicKey,{algorithms:['ES256']});}catch{alteredRejected=true;}
   const other=await generateKeyPair('ES384');try{await compactVerify(input.compact,other.publicKey,{algorithms:['ES256']});}catch{curveRejected=true;}
   const rfc='eyJhbGciOiJFUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.DtEhU3ljbEg8L38VWAfUAqOyKAM6-Xx-F4GawxaepmXFCgfTjDxw5djxLa8ISlSApmWQxfKTUJqPP3-Kg6NU1Q';
   const jwk={kty:'EC',crv:'P-256',x:'f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU',y:'x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0'};
   await compactVerify(rfc,await importJWK(jwk,'ES256'),{algorithms:['ES256']});
   send({verified:true,alteredRejected,curveRejected,rfcVerified:true,payload:Buffer.from(good.payload).toString('base64url')});
  }else if(input.kind==='sign'){
   const compact=await new CompactSign(Buffer.from(input.payload,'base64url')).setProtectedHeader(input.header).sign(pair.privateKey);send({compact});
  }else if(input.kind==='done'){break;}else{throw new Error('unknown_fixture_command');}
 }catch{send({error:'interop_fixture_failed'});process.exitCode=1;break;}
}
