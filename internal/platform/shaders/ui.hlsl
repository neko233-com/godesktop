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
    nointerpolation float2 shadowOrigin : TEXCOORD13;
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
    o.shadowOrigin = float2(0,0);
    if (v.kind == 7) {
        o.local = o.pixelPosition-v.uv.xy;
        o.size = v.uv.zw;
        o.uv = float2(v.padding.y,0);
        o.shadowOrigin = v.uv.xy;
    }
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

// A fixed normal-CDF approximation. A bounded amount of work keeps
// Gaussian mask shadows independent of blur size and allocates no texture.
float shadowCDF(float z) {
    float a=abs(z), t=1.0/(1.0+0.2316419*a);
    float polynomial=t*(0.319381530+t*(-0.356563782+t*(1.781477937+t*(-1.821255978+t*1.330274429))));
    float tail=0.3989422804014327*exp(-0.5*a*a)*polynomial;
    return z>=0 ? 1.0-tail : tail;
}
float shadowInterval(float samplePoint, float lower, float upper, float sigma) {
    return max(0.0,shadowCDF((upper-samplePoint)/sigma)-shadowCDF((lower-samplePoint)/sigma));
}
float shadowSumError(float a, float b, float sum) {
    precise float part=sum-a;
    precise float error=(a-(sum-part))+(b-part);
    return error;
}
float shadowProductError(float a, float b, float product) {
    // Split a float mantissa into exact high/low products. Unlike mad this
    // compensation does not depend on fused arithmetic being available.
    precise float splitA=4097.0*a, splitB=4097.0*b;
    precise float highA=splitA-(splitA-a), highB=splitB-(splitB-b);
    precise float lowA=a-highA, lowB=b-highB;
    precise float error=((highA*highB-product)+highA*lowB+lowA*highB)+lowA*lowB;
    return error;
}
float shadowCircleResidual(float x, float y, float radius) {
    // Compensated circle equation at the query, not a rounded absolute inset.
    // Only this small residual needs extra precision when sigma is tiny.
    precise float dx=radius-x, dy=radius-y;
    precise float ex=shadowSumError(radius,-x,dx), ey=shadowSumError(radius,-y,dy);
    precise float rr=radius*radius, xx=dx*dx, yy=dy*dy;
    precise float first=rr-xx, second=first-yy;
    precise float tail=shadowProductError(radius,radius,rr)-shadowProductError(dx,dx,xx)-shadowProductError(dy,dy,yy);
    tail=tail-2*dx*ex-2*dy*ey-ex*ex-ey*ey;
    tail=tail+shadowSumError(rr,-xx,first)+shadowSumError(first,-yy,second);
    return second+tail;
}
float shadowCap(float2 samplePoint, float2 size, float radius, float sigma, float lower, float upper, bool top) {
    // Integrate a rounded cap in y. The horizontal Gaussian integral is exact;
    // each cap uses twelve fixed Gauss-Legendre points, truncated at four sigma.
    // Integrate in standard deviations relative to the query. Small steps must
    // not be rounded by adding them to a large absolute cap coordinate.
    lower=max((lower-samplePoint.y)/sigma,-4.0); upper=min((upper-samplePoint.y)/sigma,4.0);
    if(upper<=lower) return 0.0;
    const float nodes[12]={-0.981560634,-0.904117256,-0.769902674,-0.587317954,-0.367831499,-0.125233409,0.125233409,0.367831499,0.587317954,0.769902674,0.904117256,0.981560634};
    const float weights[12]={0.0471753364,0.106939326,0.160078329,0.203167427,0.233492537,0.249147046,0.249147046,0.233492537,0.203167427,0.160078329,0.106939326,0.0471753364};
    float middle=(lower+upper)*0.5, interval=(upper-lower)*0.5, total=0.0;
    float edge=top?samplePoint.y:size.y-samplePoint.y;
    float residual=shadowCircleResidual(samplePoint.x,edge,radius);
    [unroll] for(uint i=0;i<12;i++) {
        float normalized=middle+interval*nodes[i], delta=(top?1.0:-1.0)*sigma*normalized;
        float root=sqrt(max(0.0,edge*(2*radius-edge)+delta*(2*(radius-edge)-delta)));
        float denominator=root+(radius-samplePoint.x);
        float circle=(delta*(2*(radius-edge))+residual)-delta*delta;
        float left=samplePoint.x<radius && denominator>0.0?circle/denominator:samplePoint.x-radius+root;
        float right=size.x-samplePoint.x-radius+root;
        float horizontal=max(0.0,shadowCDF(right/sigma)-shadowCDF(-left/sigma));
        total+=weights[i]*horizontal*exp(-0.5*normalized*normalized);
    }
    return total*interval*0.3989422804014327;
}
float shadowBoxInterval(float samplePoint, float lower, float upper, float pixelWidth) {
    return max(0.0,min(upper,samplePoint+pixelWidth*0.5)-max(lower,samplePoint-pixelWidth*0.5))/pixelWidth;
}
float shadowBoxCap(float2 samplePoint, float2 size, float radius, float2 pixelSize, float lower, float upper, bool top) {
    lower=max(lower,samplePoint.y-pixelSize.y*0.5); upper=min(upper,samplePoint.y+pixelSize.y*0.5);
    if(upper<=lower) return 0.0;
    const float nodes[8]={-0.960289856,-0.796666477,-0.525532410,-0.183434642,0.183434642,0.525532410,0.796666477,0.960289856};
    const float weights[8]={0.101228536,0.222381034,0.313706646,0.362683783,0.362683783,0.313706646,0.222381034,0.101228536};
    float middle=(lower+upper)*0.5, interval=(upper-lower)*0.5, total=0.0;
    [unroll] for(uint i=0;i<8;i++) {
        float y=middle+interval*nodes[i], edge=top?y:size.y-y;
        float inset=radius-sqrt(max(0.0,edge*(2*radius-edge)));
        total+=weights[i]*shadowBoxInterval(samplePoint.x,inset,size.x-inset,pixelSize.x);
    }
    return total*interval/pixelSize.y;
}
float shadowCoverage(float2 local, float2 size, float radius, float sigma) {
    // Keep point/edge coordinates local: subtracting half of a large surface
    // first destroys the tiny displacement of a small-sigma sample at its edge.
    float2 samplePoint=local;
    radius=clamp(radius,0.0,min(size.x,size.y)*0.5);
    if(sigma<=0.0) {
        // A sharp shadow covers the actual device-pixel box. This handles
        // subpixel silhouettes and rectangle corners without changing the
        // existing ordinary-shape signed-distance antialiasing contract.
        float2 pixelSize=max(fwidth(local),float2(0.0001,0.0001));
        float sharp=shadowBoxInterval(samplePoint.x,0.0,size.x,pixelSize.x)*shadowBoxInterval(samplePoint.y,radius,size.y-radius,pixelSize.y);
        if(radius>0.0) {
            sharp+=shadowBoxCap(samplePoint,size,radius,pixelSize,0.0,radius,true);
            sharp+=shadowBoxCap(samplePoint,size,radius,pixelSize,size.y-radius,size.y,false);
        }
        return saturate(sharp);
    }
    // The mask is symmetric. Evaluate at the nearest top/left edge so tiny
    // sigma cap samples do not add small offsets to a large bottom coordinate.
    samplePoint=float2(min(local.x,size.x-local.x),min(local.y,size.y-local.y));
    float coverage=shadowInterval(samplePoint.x,0.0,size.x,sigma)*shadowInterval(samplePoint.y,radius,size.y-radius,sigma);
    if(radius>0.0) {
        coverage+=shadowCap(samplePoint,size,radius,sigma,0.0,radius,true);
        coverage+=shadowCap(samplePoint,size,radius,sigma,size.y-radius,size.y,false);
    }
    return saturate(coverage);
}
float4 fragment_main(Out input) : SV_Target {
    if (any(input.pixelPosition<input.clip.xy) || any(input.pixelPosition>=input.clip.xy+input.clip.zw)) discard;
    float alpha;
    float coverage=roundedCoverage(input.pixelPosition,input.rounded0,input.roundedRadii.x)*roundedCoverage(input.pixelPosition,input.rounded1,input.roundedRadii.y)*roundedCoverage(input.pixelPosition,input.rounded2,input.roundedRadii.z)*roundedCoverage(input.pixelPosition,input.rounded3,input.roundedRadii.w);
    if(coverage<=0) discard;
    if (input.kind == 7) {
        // Rasterizer subpixel snapping must not move a tiny Gaussian query.
        // The private frame uniform carries the real drawable pixels per DIP.
        precise float2 local=input.uv.x>0?input.position.xy/framePadding.x-input.shadowOrigin:input.local;
        alpha=shadowCoverage(local,input.size,input.radius,input.uv.x)*input.color.a;
        return float4(input.color.rgb*alpha,alpha)*coverage;
    }
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
