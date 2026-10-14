#pragma once
// One renderer for ESP32 firmware and the macOS preview helper. No host pixels
// or drawing commands cross USB: layout and font rasterization live here.
#include <ArduinoJson.h>
#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <cmath>
#include "banner_assets.h"

namespace dashboard {
constexpr int Width=1404, Height=1872, NativeWidth=1872, NativeHeight=1404;
constexpr size_t FrameSize=size_t(Width)*Height/2;
constexpr size_t FontSize=65536u*33u, MaxPayload=192u*1024u;
constexpr int TaskRows=9;
// Synthetic semibold in output pixels, not font cells. Horizontal expansion
// preserves thin gaps between stacked CJK strokes and all existing text metrics.
// Shared by device and Mac preview.
constexpr int TextStroke=2;
constexpr int TitleStroke=6;
constexpr int Margin=64, ContentWidth=1276, ColumnWidth=638;
constexpr int BannerX=64, BannerY=512, BannerWidth=704, BannerHeight=176;
constexpr double NoteAngle=0.0;
constexpr const char* RendererVersion="glance-3";
static_assert(BannerWidth==banner_assets::Width && BannerHeight==banner_assets::Height,"banner dimensions");

inline bool stringField(JsonVariantConst v, size_t max) {
  return v.is<const char*>() && strlen(v.as<const char*>())<=max;
}
inline bool state(JsonVariantConst v) {
  return v=="running" || v=="completed" || v=="failed" || v=="interrupted";
}
inline bool source(JsonVariantConst v) {
  return v.is<JsonObjectConst>() && (v["state"]=="fresh" || v["state"]=="stale" || v["state"]=="unavailable") && stringField(v["last_success"],48);
}
inline bool bannerID(JsonVariantConst v) {
  return v=="unknown" || (v.is<const char*>() && banner_assets::find(v.as<const char*>()).data);
}
inline bool percentage(JsonVariantConst v) {
  return v.is<double>() && std::isfinite(v.as<double>()) && v.as<double>()>=0 && v.as<double>()<=100;
}
inline bool valid(JsonVariantConst s) {
  if (!s.is<JsonObjectConst>() || s["version"]!=2 || !s["privacy"].is<bool>() || !source(s["quota_status"]) || !source(s["activity_status"])) return false;
  auto d=s["display"];
  if (!stringField(d["title"],64)) return false;
  if (!d.is<JsonObjectConst>() || !stringField(d["date"],16) || !stringField(d["time"],8) || !stringField(d["weekday"],16) || !bannerID(d["banner_id"]) || !d["notes"].is<JsonArrayConst>() || d["notes"].size()!=4) return false;
  for (auto note:d["notes"].as<JsonArrayConst>()) if (!stringField(note,640)) return false;
  if (!s["quota_pairs"].is<JsonArrayConst>() || s["quota_pairs"].size()>64) return false;
  for (auto pair:s["quota_pairs"].as<JsonArrayConst>()) {
    if (!stringField(pair["bucket"],192) || !bannerID(pair["banner_id"])) return false;
    if (!pair["weekly_only"].is<bool>()) return false;
    if (pair["weekly_only"]==true && (!pair["five_hour"].isNull() || pair["weekly"].isNull() || s["quota_status"]["state"]=="unavailable")) return false;
    for (const char* key:{"five_hour","weekly"}) {
      auto w=pair[key];
      if (!w.isNull() && (!w.is<JsonObjectConst>() || !stringField(w["resets_at"],48) || !percentage(w["used_percent"]))) return false;
    }
  }
  if (!s["windows"].is<JsonArrayConst>() || s["windows"].size()>64 || !s["tasks"].is<JsonArrayConst>() || s["tasks"].size()>256) return false;
  if (!s["counts"].is<JsonArrayConst>() || s["counts"].size()!=4 || !s["omitted_tasks"].is<unsigned>()) return false;
  if (!s["reset_credits"].isNull() && !s["reset_credits"].is<unsigned>()) return false;
  auto expiry=s["reset_expiry"];
  if (!expiry.is<JsonObjectConst>() || !stringField(expiry["date"],10) || !stringField(expiry["time"],5) || !expiry["partial"].is<bool>()) return false;
  if (expiry["state"]!="unknown" && expiry["state"]!="none" && expiry["state"]!="never" && expiry["state"]!="dated" && expiry["state"]!="unsupported") return false;
  if (expiry["state"]=="dated" && (strlen(expiry["date"])!=10 || strlen(expiry["time"])!=5)) return false;
  if (s["privacy"]==true && s["tasks"].size()!=0) return false;
  for (JsonVariantConst w:s["windows"].as<JsonArrayConst>()) {
    if (!stringField(w["name"],192) || !stringField(w["bucket"],192) || !stringField(w["resets_at"],48) || !percentage(w["used_percent"])) return false;
  }
  for (JsonVariantConst t:s["tasks"].as<JsonArrayConst>()) {
    if (!stringField(t["title"],384) || !state(t["state"]) || !stringField(t["at"],48) || !t["stale"].is<bool>()) return false;
  }
  const char* names[]={"running","completed","failed","interrupted"};
  unsigned i=0;
  for (JsonVariantConst c:s["counts"].as<JsonArrayConst>()) {
    if (c["state"]!=names[i++] || !c["count"].is<unsigned>() || !c["stale"].is<unsigned>() || c["stale"].as<unsigned>()>c["count"].as<unsigned>()) return false;
  }
  return true;
}
inline const char* health(JsonVariantConst v) {
  return v=="fresh"?"已同步":v=="stale"?"数据已过期":"暂不可用";
}
inline const char* taskState(JsonVariantConst v) {
  return v=="running"?"执行中":v=="failed"?"失败":v=="interrupted"?"已中断":"本轮完成";
}
inline uint32_t rune(const char*& s) {
  uint8_t a=uint8_t(*s++);
  if (a<128) return a;
  int more=a>=0xF0&&a<=0xF4?3:a>=0xE0&&a<=0xEF?2:a>=0xC2&&a<=0xDF?1:0;
  if (!more) return 0xFFFD;
  uint32_t cp=a&((1u<<(6-more))-1);
  for (int i=0;i<more;++i) {
    uint8_t b=uint8_t(*s);
    if ((b&0xC0)!=0x80) return 0xFFFD;
    ++s; cp=(cp<<6)|(b&63);
  }
  if (cp>0xFFFF || (cp>=0xD800&&cp<=0xDFFF) || (more==1&&cp<128) || (more==2&&cp<2048)) return 0xFFFD;
  return cp;
}
class Canvas {
  uint8_t* pixels;
  const uint8_t* font;
public:
  Canvas(uint8_t* p,const uint8_t* f):pixels(p),font(f) {}
  void clear() { memset(pixels,255,FrameSize); }
  void pixel(int x,int y,uint8_t gray) {
    if (x<0||y<0||x>=Width||y>=Height) return;
    int nx=NativeWidth-1-y, ny=x;
    size_t i=(size_t(ny)*NativeWidth+nx)/2;
    if (nx&1) pixels[i]=(pixels[i]&0xF0)|(gray&15);
    else pixels[i]=(pixels[i]&15)|((gray&15)<<4);
  }
  void rect(int x,int y,int w,int h,uint8_t gray) {
    for (int yy=std::max(0,y);yy<std::min(Height,y+h);++yy)
      for (int xx=std::max(0,x);xx<std::min(Width,x+w);++xx) pixel(xx,yy,gray);
  }
  void disk(int x,int y,int r,uint8_t gray) {
    for(int yy=-r;yy<=r;++yy) {
      int dx=int(sqrt(double(r*r-yy*yy)));
      rect(x-dx,y+yy,2*dx+1,1,gray);
    }
  }
  void line(int x0,int y0,int x1,int y1,int stroke=3,uint8_t gray=0) {
    int dx=abs(x1-x0),sx=x0<x1?1:-1,dy=-abs(y1-y0),sy=y0<y1?1:-1,err=dx+dy;
    for (;;) {
      disk(x0,y0,stroke/2,gray);
      if(x0==x1 && y0==y1) break;
      int e=2*err; if(e>=dy){err+=dy;x0+=sx;} if(e<=dx){err+=dx;y0+=sy;}
    }
  }
  void capsule(int x,int y,int w,int h,uint8_t gray) {
    if(w<=0 || h<=0) return;
    int r=std::min(w,h)/2;
    for(int yy=0;yy<h;++yy) {
      double dy=yy<r?r-yy-0.5:yy>=h-r?yy-(h-r)+0.5:0;
      int inset=dy>0?int(ceil(r-sqrt(std::max(0.,double(r*r)-dy*dy)))):0;
      rect(x+inset,y+yy,w-2*inset,1,gray);
    }
  }
  const uint8_t* glyph(uint32_t cp) const {
    if (cp>0xFFFF || !font[cp*33]) cp=0xFFFD;
    return font+cp*33;
  }
  // Center by visible ink, not the font cell (important for single digits).
  void centeredText(const char* s,int cx,int cy,int size,uint8_t gray=0,bool centerX=true) {
    int advance=0,left=Width,right=-1,top=size,bottom=-1;
    for(const char* p=s;*p;) {
      const uint8_t* g=glyph(rune(p)); int gw=g[0]==8?8:16,w=gw*size/16;
      for(int y=0;y<size;++y) {
        int row=y*16/size; uint16_t bits=(uint16_t(g[1+row*2])<<8)|g[2+row*2];
        for(int x=0;x<w;++x) if(bits&(0x8000u>>(x*gw/w))) {
          left=std::min(left,advance+x); right=std::max(right,advance+std::min(w-1,x+TextStroke));
          top=std::min(top,y); bottom=std::max(bottom,y);
        }
      }
      advance+=w;
    }
    if(right>=left) text(s,centerX?cx-(left+right)/2:cx,cy-(top+bottom)/2,size,advance,gray);
  }
  void text(const char* s,int x,int y,int size,int maxWidth,uint8_t gray=0,int stroke=TextStroke) {
    if (!s || size<=0 || maxWidth<=0) return;
    const int right=std::min(Width,x+maxWidth);
    const char* probe=s; int measured=0;
    while(*probe && measured<=right-x) {
      uint32_t cp=rune(probe); if(cp<32 || (cp>=127&&cp<160)) cp=' ';
      measured+=(glyph(cp)[0]==8?8:16)*size/16;
    }
    const bool truncate=measured>right-x;
    while (*s) {
      uint32_t cp=rune(s);
      if (cp<32 || (cp>=127&&cp<160)) cp=' ';
      const uint8_t* g=glyph(cp);
      int gw=g[0]==8?8:16, w=gw*size/16;
      bool last=truncate && x+w>right-size;
      if (last) { g=glyph(0x2026); gw=16; w=size; }
      if (x+w>right) break;
      for (int yy=0;yy<size;++yy) {
        int row=yy*16/size;
        uint16_t bits=(uint16_t(g[1+row*2])<<8)|g[2+row*2];
        for (int xx=0;xx<w;++xx) if (bits & (0x8000u>>(xx*gw/w))) {
          for (int dx=0;dx<=stroke && xx+dx<w;++dx)
            pixel(x+xx+dx,y+yy,gray);
        }
      }
      x+=w;
      if (last) break;
    }
  }
};
inline unsigned detailPageCount(JsonVariantConst s) {
  unsigned q=std::max(1u,(unsigned(s["windows"].size())+2)/3);
  unsigned t=s["privacy"]==true?1:std::max(1u,(unsigned(s["tasks"].size())+TaskRows-1)/TaskRows);
  return std::max(q,t);
}
inline int remainingPercent(JsonVariantConst window) {
  return int(std::round(100-window["used_percent"].as<double>()));
}
inline void renderDetails(uint8_t* pixels,const uint8_t* font,JsonVariantConst s,unsigned page=0) {
  Canvas c(pixels,font); c.clear(); char buf[512];
  c.text("Glance",64,64,48,850);
  c.text("CODEX / USB",1032,88,24,308,4);
  c.rect(64,150,1276,4,0);
  c.text("官方额度",64,190,32,720);
  c.text(health(s["quota_status"]["state"]),1040,202,24,300,5);
  auto windows=s["windows"].as<JsonArrayConst>();
  unsigned qpages=std::max(1u,(unsigned(windows.size())+2)/3), qp=page%qpages;
  unsigned qstart=qp*3, n=std::min(3u,unsigned(windows.size())-qstart);
  if (!n) c.text("等待额度数据",64,350,32,1276,4);
  for (unsigned i=0;i<n;++i) {
    auto w=windows[qstart+i]; int cw=(1276-32*(int(n)-1))/int(n), x=64+int(i)*(cw+32);
    const char* bucket=w["bucket"], *name=w["name"];
    snprintf(buf,sizeof(buf),"%s%s%s",bucket,*bucket?" · ":"",name);
    c.text(buf,x,268,24,cw,3);
    int remaining=remainingPercent(w);
    snprintf(buf,sizeof(buf),"%d%%",remaining); c.text(buf,x,316,80,cw-80);
    c.text("剩余",x+cw-64,370,24,64,5);
    snprintf(buf,sizeof(buf),"已用 %d%%",100-remaining); c.text(buf,x,422,24,cw,4);
    snprintf(buf,sizeof(buf),"重置 %s",w["resets_at"].as<const char*>()); c.text(buf,x,462,24,cw,4);
    c.rect(x,510,cw,12,13); c.rect(x,510,cw*remaining/100,12,2);
  }
  if (!s["reset_credits"].isNull()) {
    snprintf(buf,sizeof(buf),"可用重置额度 %u",s["reset_credits"].as<unsigned>()); c.text(buf,64,552,24,800,3);
  }
  if (qpages>1) { snprintf(buf,sizeof(buf),"额度 %u / %u 页",qp+1,qpages); c.text(buf,1020,552,24,320,4); }
  c.rect(64,606,1276,2,12);
  c.text("任务动态",64,642,32,900);
  snprintf(buf,sizeof(buf),"最近 24 小时 · %s%s",health(s["activity_status"]["state"]),s["privacy"]==true?" · 隐私模式":"");
  c.text(buf,64,690,24,1276,5);
  if (s["privacy"]==true) {
    auto counts=s["counts"].as<JsonArrayConst>();
    for (unsigned i=0;i<4;++i) {
      auto entry=counts[i]; int y=766+int(i)*188;
      c.text(taskState(entry["state"]),64,y,32,900);
      snprintf(buf,sizeof(buf),"%u",entry["count"].as<unsigned>()); c.text(buf,1100,y,64,240);
      if (entry["stale"].as<unsigned>()) { snprintf(buf,sizeof(buf),"其中 %u 项状态已过期",entry["stale"].as<unsigned>()); c.text(buf,64,y+58,24,1000,5); }
      c.rect(64,y+116,1276,2,13);
    }
  } else {
    auto tasks=s["tasks"].as<JsonArrayConst>();
    unsigned tpages=std::max(1u,(unsigned(tasks.size())+TaskRows-1)/TaskRows), tp=page%tpages;
    if (tasks.size()==0) c.text("暂无可验证的近期任务活动",64,800,32,1276,4);
    for (unsigned i=0;i<TaskRows && tp*TaskRows+i<tasks.size();++i) {
      auto t=tasks[tp*TaskRows+i]; int y=764+int(i)*98;
      if (t["state"]=="running" && t["stale"]==false) c.rect(64,y,8,70,0);
      c.text(t["stale"]==true?"状态已过期":taskState(t["state"]),92,y,24,800,3);
      c.text(t["at"],1076,y,24,264,5);
      c.text(t["title"],92,y+38,32,1248);
      c.rect(92,y+80,1248,1,13);
    }
    snprintf(buf,sizeof(buf),"任务 %u / %u 页 · 共 %u 项",tp+1,tpages,unsigned(tasks.size())+s["omitted_tasks"].as<unsigned>());
    c.text(buf,64,1680,24,820,4);
    if (s["omitted_tasks"].as<unsigned>()) { snprintf(buf,sizeof(buf),"另有 %u 项未载入",s["omitted_tasks"].as<unsigned>()); c.text(buf,980,1680,24,360,4); }
  }
  c.rect(64,1734,1276,2,0);
  if (s["quota_status"]["state"]!="fresh") {
    snprintf(buf,sizeof(buf),"额度最后成功 %s",s["quota_status"]["last_success"].as<const char*>()); c.text(buf,64,1760,24,680,4);
  } else c.text("USB 数据同步 · 设备本地绘图",64,1760,24,680,4);
  if (s["activity_status"]["state"]!="fresh") {
    snprintf(buf,sizeof(buf),"任务最后成功 %s",s["activity_status"]["last_success"].as<const char*>()); c.text(buf,760,1760,24,580,4);
  } else { snprintf(buf,sizeof(buf),"详情 %u / %u · 白色按键翻页",page%detailPageCount(s)+1,detailPageCount(s)); c.text(buf,760,1760,24,580,4); }
  c.text("任务状态反映最近一轮执行，不代表整个任务已验收完成。",64,1820,20,1276,6);
}

inline void resetIcon(Canvas& c,int x,int y) {
  // Two opposing 140-degree arcs, filled tangent arrowheads, rounded tails.
  // 4x4 coverage sampling smooths the outline directly into the Gray4 buffer.
  // Geometry is firmware-local; no image or extra frame buffer is transmitted.
  for(int yy=-80;yy<=80;++yy) for(int xx=-80;xx<=80;++xx) {
    int ink=0;
    for(int sy=0;sy<4;++sy) for(int sx=0;sx<4;++sx) {
      double u=xx+(sx+0.5)/4-0.5,v=yy+(sy+0.5)/4-0.5;
      bool hit=false;
      for(int side=0;side<2;++side,u=-u,v=-v) {
        double r2=u*u+v*v;
        bool arc=r2>=57.5*57.5 && r2<=66.5*66.5 && v<=0.466307658*u && v<=-0.267949192*u;
        double tailX=u+56.1910828,tailY=v+26.2023322;
        double tangent=0.258819045*u+0.965925826*v,normal=0.965925826*u-0.258819045*v;
        bool arrow=tangent>=-18 && tangent<=8 && fabs(normal-62)<=(8-tangent)*14/26;
        hit=hit || arc || tailX*tailX+tailY*tailY<=4.5*4.5 || arrow;
      }
      if(hit) ++ink;
    }
    if(ink) c.pixel(x+xx,y+yy,uint8_t(15-(ink*15+8)/16));
  }
}

// Prepared text positions are bounded stack data, not a temporary image. The
// inverse rotation below samples both note geometry and glyphs in one pass,
// avoiding forward-mapping holes and large scratch allocations on the ESP32.
struct NoteGlyph { uint32_t cp; int x,y,w; };
inline void note(Canvas& c,const char* text,int cx,int cy,double degrees) {
  // Center seven full-width character cells; unused wrapping space must not
  // accumulate on the right. Short lines remain aligned to this shared inset.
  constexpr int W=584,H=452,Size=64,TextWidth=7*Size,Left=(W-TextWidth)/2,Top=80,LineHeight=84,Lines=4;
  static_assert(Left==W-Left-TextWidth,"note side insets must match");
  NoteGlyph glyphs[160]; int count=0,x=0,row=0; bool truncated=false;
  const char* s=text;
  while(s && *s && count<160) {
    uint32_t cp=rune(s);
    if(cp=='\n') {x=0; if(++row>=Lines){truncated=*s;break;} continue;}
    if(cp<32 || (cp>=127&&cp<160)) cp=' ';
    int w=c.glyph(cp)[0]==8?Size/2:Size;
    if(x+w>TextWidth) {x=0;++row;}
    if(row>=Lines) {truncated=true;break;}
    glyphs[count++]={cp,Left+x,Top+row*LineHeight,w}; x+=w;
  }
  if(s && *s) truncated=true;
  if(truncated && count) {
    while(count>0 && glyphs[count-1].y==Top+(Lines-1)*LineHeight && glyphs[count-1].x>Left+TextWidth-Size) --count;
    if(count) glyphs[count-1]={0x2026,glyphs[count-1].x,glyphs[count-1].y,Size};
  }
  double angle=degrees*3.14159265359/180;
  const int cs=int(lround(cos(angle)*65536)),sn=int(lround(sin(angle)*65536));
  auto floorFixed=[](int n) {return n>=0?n/65536:-((-n+65535)/65536);};
  for(int yy=-278;yy<=278;++yy) {
    int u=-318*cs+yy*sn+W/2*65536,v=318*sn+yy*cs+H/2*65536;
    for(int xx=-318;xx<=318;++xx,u+=cs,v-=sn) {
    int lx=floorFixed(u),ly=floorFixed(v);
    int gray=-1;
    if(lx>=14 && lx<W+14 && ly>=16 && ly<H+16) gray=0;
    if(lx>=0 && lx<W && ly>=0 && ly<H) {
      gray=(lx<3||lx>=W-3||ly<3||ly>=H-3)?0:15;
      if(lx>=Left && lx<Left+TextWidth && ly>=Top && ly<Top+Lines*LineHeight) {
        for(int i=0;i<count;++i) {
          const auto& g=glyphs[i];
          if(lx<g.x||lx>=g.x+g.w||ly<g.y||ly>=g.y+Size) continue;
          const uint8_t* bitmap=c.glyph(g.cp); int gw=bitmap[0]==8?8:16,fr=(ly-g.y)*16/Size;
          uint16_t bits=(uint16_t(bitmap[1+fr*2])<<8)|bitmap[2+fr*2];
          for(int dx=0;dx<=TextStroke;++dx) if(lx-g.x-dx>=0 && (bits&(0x8000u>>((lx-g.x-dx)*gw/g.w)))) gray=0;
        }
      }
    }
    // Temporary tape is code-drawn, separate from the pending banner artwork.
    if(ly>=-30 && ly<32) {
      int edge=(ly/5)%2?2:0,tx=W/2-62+edge;
      if(lx>=tx && lx<tx+124) gray=(lx<tx+3||lx>=tx+121||ly<-27||ly>=29)?0:15;
    }
    if(gray>=0) c.pixel(cx+xx,cy+yy,uint8_t(gray));
    }
  }
}

inline void quotaColumn(Canvas& c,JsonVariantConst pair,bool weekly,bool fullWidth=false) {
  int x=fullWidth?Margin:weekly?734:64;
  int W=fullWidth?ContentWidth:606; char buf[192];
  c.text(weekly?"7 天额度":"5 小时额度",x,274,48,320);
  auto w=pair[weekly?"weekly":"five_hour"];
  if(w.isNull()) snprintf(buf,sizeof(buf),"--%%");
  else snprintf(buf,sizeof(buf),"%d%%",remainingPercent(w));
  c.text(buf,x+W-int(strlen(buf))*44,248,88,220,w.isNull()?6:0);
  c.capsule(x,360,W,54,w.isNull()?8:0); c.capsule(x+3,363,W-6,48,15);
  if(w.isNull()) {c.text("暂无额度数据",x,440,30,W,6);return;}
  int fill=(W-14)*remainingPercent(w)/100;
  c.capsule(x+7,367,fill,40,0);
  snprintf(buf,sizeof(buf),"%s 重置",w["resets_at"].as<const char*>());
  c.text(buf,x,440,30,W);
}

inline void resetCredits(Canvas& c,JsonVariantConst s) {
  if(s["reset_expiry"]["state"]=="unsupported") return;
  char buf[64];
  resetIcon(c,896,600);
  if(!s["reset_credits"].isNull()) {
    snprintf(buf,sizeof(buf),"%u",s["reset_credits"].as<unsigned>());
  } else snprintf(buf,sizeof(buf),"--");
  int size=strlen(buf)==1?80:std::min(64,176/int(strlen(buf)));
  c.centeredText(buf,896,600,size);
  auto expiry=s["reset_expiry"];
  if(expiry["state"]=="dated") {
    c.text(expiry["date"],1006,542,44,334);
    snprintf(buf,sizeof(buf),"%s 过期",expiry["time"].as<const char*>());
    c.text(buf,1006,604,40,334);
    if(expiry["partial"]==true) c.text("已知最近到期",1006,658,20,334,5);
  } else {
    // Keep the existing left edge; align the visible ink to the icon's center.
    c.centeredText(expiry["state"]=="none"?"暂无重置机会":expiry["state"]=="never"?"无到期限制":"到期时间未知",1006,600,28,5,false);
  }
}

inline unsigned homePageCount(JsonVariantConst s) { return std::max(1u,unsigned(s["quota_pairs"].size())); }
inline unsigned pageCount(JsonVariantConst s) { return homePageCount(s)+detailPageCount(s); }
inline void banner(Canvas& c,const char* id) {
  auto asset=banner_assets::find(id);
  if(!asset.data) {
    c.text("等待额度数据",BannerX+208,BannerY+64,36,480,6);
    return;
  }
  // High nibble is run length minus one; low nibble is the Gray4 pixel.
  // Decode directly from flash into the existing frame, without a second image.
  int p=0;
  for(size_t i=0;i<asset.size && p<BannerWidth*BannerHeight;++i) {
    uint8_t b=asset.data[i]; int n=(b>>4)+1;
    for(int j=0;j<n && p<BannerWidth*BannerHeight;++j,++p)
      c.pixel(BannerX+p%BannerWidth,BannerY+p/BannerWidth,b&15);
  }
}
inline void render(uint8_t* pixels,const uint8_t* font,JsonVariantConst s,unsigned page=0) {
  page%=pageCount(s);
  if(page>=homePageCount(s)) {renderDetails(pixels,font,s,page-homePageCount(s));return;}
  Canvas c(pixels,font); c.clear(); char buf[192];
  auto display=s["display"];
  c.text(display["title"],64,24,184,800,0,TitleStroke);
  // Calendar only: minutes must not independently trigger an e-paper refresh.
  c.text(display["date"],1120,50,44,220);
  c.text(display["weekday"],1100,107,80,240);
  c.rect(Margin,228,ContentWidth,3,0);
  auto pair=s["quota_pairs"][page];
  if(s["quota_status"]["state"]!="fresh") c.text(health(s["quota_status"]["state"]),1050,234,24,290,5);
  if(pair["weekly_only"]==true) quotaColumn(c,pair,true,true);
  else {quotaColumn(c,pair,false); quotaColumn(c,pair,true);}
  banner(c,pair.isNull()?display["banner_id"].as<const char*>():pair["banner_id"].as<const char*>());
  resetCredits(c,s);
  auto notes=display["notes"].as<JsonArrayConst>();
  note(c,notes[0],367,986,NoteAngle); note(c,notes[1],1037,986,NoteAngle);
  note(c,notes[2],367,1514,NoteAngle); note(c,notes[3],1037,1514,NoteAngle);
  snprintf(buf,sizeof(buf),"更新于 %s %s",display["date"].as<const char*>(),display["time"].as<const char*>());
  c.text(buf,972,1810,32,368,4);
}
} // namespace dashboard
