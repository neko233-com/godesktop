// Original godesktop UI shader. The instance ABI mirrors gpu_scene.h.
struct Instance {
    float4 bounds;
    float4 clip;
    float4 color;
    float4 uv;
    float radius;
    uint kind;
    float2 padding;
    float4 roundedBounds[4];
    float4 roundedRadii;
};

StructuredBuffer<Instance> instances : register(t0);
Texture2D<float4> glyphs[16] : register(t1);
Texture2D<float4> bitmap : register(t17);
SamplerState glyphSampler : register(s0);
cbuffer Frame : register(b0) { float2 viewport; float2 framePadding; };

struct Out {
    float4 position : SV_Position;
    float2 pixelPosition : TEXCOORD0;
    float2 local : TEXCOORD1;
    nointerpolation float2 size : TEXCOORD2;
    float2 uv : TEXCOORD3;
    nointerpolation float4 color : COLOR0;
    nointerpolation float4 clip : TEXCOORD4;
    nointerpolation float radius : TEXCOORD5;
    nointerpolation uint kind : TEXCOORD6;
    nointerpolation uint glyphSlot : TEXCOORD7;
    nointerpolation float4 rounded0 : TEXCOORD8;
    nointerpolation float4 rounded1 : TEXCOORD9;
    nointerpolation float4 rounded2 : TEXCOORD10;
    nointerpolation float4 rounded3 : TEXCOORD11;
    nointerpolation float4 roundedRadii : TEXCOORD12;
};

Out vertex_main(uint vertexIndex : SV_VertexID, uint instanceIndex : SV_InstanceID) {
    const float2 corners[6] = {
        float2(0,0),float2(1,0),float2(0,1),
        float2(1,0),float2(1,1),float2(0,1)
    };
    Instance v = instances[instanceIndex];
    float2 corner = corners[vertexIndex];
    Out o;
    o.pixelPosition = v.bounds.xy + corner*v.bounds.zw;
    o.size = v.bounds.zw;
    o.local = corner*o.size;
    o.radius = v.radius;
    if (v.kind == 3) {
        float extent = max(0.001, length(v.bounds.zw));
        float2 axis = v.bounds.zw / extent;
        o.pixelPosition = v.bounds.xy + corner.x*extent*axis + (corner.y-0.5)*v.radius*float2(-axis.y,axis.x);
        o.size = float2(extent,v.radius);
        o.local = corner*o.size;
        o.radius = 0;
    }
    o.position = float4(o.pixelPosition/viewport*float2(2,-2)+float2(-1,1),0,1);
    o.uv = v.uv.xy + corner*v.uv.zw;
    o.color = v.color;
    o.clip = v.clip;
    o.kind = v.kind;
    o.glyphSlot = uint(v.padding.x);
    o.rounded0=v.roundedBounds[0]; o.rounded1=v.roundedBounds[1];
    o.rounded2=v.roundedBounds[2]; o.rounded3=v.roundedBounds[3]; o.roundedRadii=v.roundedRadii;
    return o;
}

float4 glyphSample(uint slot, float2 uv) {
    // A wave can contain several glyph/page instances. Mark that descriptor
    // index explicitly, keeping GPU-validation instrumentation to one sample.
    return glyphs[NonUniformResourceIndex(min(slot,15u))].SampleLevel(glyphSampler,uv,0);
}

float roundedCoverage(float2 pixel, float4 bounds, float radius) {
    if(radius<=0) return 1.0;
    float2 q=abs(pixel-bounds.xy-bounds.zw*0.5)-(bounds.zw*0.5-radius);
    float distance=length(max(q,0.0))+min(max(q.x,q.y),0.0)-radius;
    float softness=max(fwidth(distance)*0.5,0.0001);
    return 1.0-smoothstep(-softness,softness,distance);
}
float4 fragment_main(Out input) : SV_Target {
    if (any(input.pixelPosition<input.clip.xy) || any(input.pixelPosition>=input.clip.xy+input.clip.zw)) discard;
    float alpha;
    float coverage=roundedCoverage(input.pixelPosition,input.rounded0,input.roundedRadii.x)*roundedCoverage(input.pixelPosition,input.rounded1,input.roundedRadii.y)*roundedCoverage(input.pixelPosition,input.rounded2,input.roundedRadii.z)*roundedCoverage(input.pixelPosition,input.rounded3,input.roundedRadii.w);
    if(coverage<=0) discard;
    if (input.kind == 6) {
        return glyphSample(input.glyphSlot,input.uv)*input.color.a*coverage;
    }
    if (input.kind == 5) {
        float4 rgba = bitmap.SampleLevel(glyphSampler,input.uv,0);
        return float4(rgba.rgb*input.color.rgb*input.color.a,rgba.a*input.color.a)*coverage;
    }
    if (input.kind == 2) {
        alpha = glyphSample(input.glyphSlot,input.uv).r*input.color.a;
    } else {
        float2 q = abs(input.local-input.size*0.5)-(input.size*0.5-input.radius);
        float distance = length(max(q,0.0))+min(max(q.x,q.y),0.0)-input.radius;
        float softness = max(fwidth(distance)*0.5,0.0001);
        alpha = (1.0-smoothstep(-softness,softness,distance))*input.color.a;
    }
    return float4(input.color.rgb*alpha,alpha)*coverage;
}
