#ifndef GODESKTOP_GPU_SHADER_METAL_H
#define GODESKTOP_GPU_SHADER_METAL_H

static NSString *const shader = @
"#include <metal_stdlib>\n"
"using namespace metal;\n"
"struct Instance { packed_float4 bounds, clip, color, uv; float radius; uint kind; packed_float2 padding; };\n"
"struct Out { float4 position [[position]]; float2 point, local, size, uv; float radius; float4 color; float4 clip; uint kind [[flat]]; };\n"
"vertex Out vertex_main(uint vertexIndex [[vertex_id]], uint index [[instance_id]], const device Instance *instances [[buffer(0)]], constant float2 &viewport [[buffer(1)]]) {\n"
"  const float2 corners[6]={float2(0,0),float2(1,0),float2(0,1),float2(1,0),float2(1,1),float2(0,1)};\n"
"  Instance v=instances[index]; float2 corner=corners[vertexIndex]; Out o;\n"
"  o.point=float2(v.bounds.xy)+corner*float2(v.bounds.zw); o.size=v.bounds.zw; o.local=corner*o.size; o.radius=v.radius;\n"
"  if(v.kind==3) { float extent=max(0.001,length(float2(v.bounds.zw))); float2 axis=float2(v.bounds.zw)/extent;\n"
"    o.point=float2(v.bounds.xy)+corner.x*extent*axis+(corner.y-0.5)*v.radius*float2(-axis.y,axis.x);\n"
"    o.size=float2(extent,v.radius); o.local=corner*o.size; o.radius=0; }\n"
"  o.position=float4(o.point/viewport*float2(2,-2)+float2(-1,1),0,1);\n"
"  o.color=v.color; o.clip=v.clip; o.uv=float2(v.uv.xy)+corner*float2(v.uv.zw); o.kind=v.kind; return o;\n"
"}\n"
"fragment float4 fragment_main(Out in [[stage_in]], texture2d<float> glyph [[texture(0)]]) {\n"
"  if(any(in.point<in.clip.xy)||any(in.point>=in.clip.xy+in.clip.zw)) discard_fragment();\n"
"  if(in.kind==2) { constexpr sampler s(filter::linear,address::clamp_to_edge); float alpha=glyph.sample(s,in.uv).r*in.color.a; return float4(in.color.rgb*alpha,alpha); }\n"
"  float2 q=abs(in.local-in.size*0.5)-(in.size*0.5-in.radius); float distance=length(max(q,0.0))+min(max(q.x,q.y),0.0)-in.radius;\n"
"  float softness=max(fwidth(distance)*0.5,0.0001); float alpha=(1.0-smoothstep(-softness,softness,distance))*in.color.a; return float4(in.color.rgb*alpha,alpha);\n"
"}\n";
#endif
