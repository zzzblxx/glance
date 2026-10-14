#include "dashboard.h"
#include <cassert>
#include <iostream>
#include <string>
#include <vector>

static uint8_t at(const std::vector<uint8_t>& frame,int x,int y) {
  const int nx=dashboard::NativeWidth-1-y;
  const size_t i=(size_t(x)*dashboard::NativeWidth+nx)/2;
  return (nx&1)?frame[i]&15:frame[i]>>4;
}

int main() {
  std::vector<uint8_t> font(dashboard::FontSize,0), frame(dashboard::FrameSize);
  // Two isolated font pixels, including one at the glyph's bottom-right edge.
  for (const unsigned cp:{unsigned('A'),0x4E2Du,0x2026u}) {
    uint8_t* glyph=font.data()+cp*33;
    glyph[0]=cp=='A'?8:16;
    glyph[1+4*2]=0x10;
    const uint16_t edge=uint16_t(0x8000u>>(glyph[0]-1));
    glyph[1+15*2]=edge>>8; glyph[2+15*2]=edge&255;
  }
  assert(dashboard::TextStroke==2);
  assert(dashboard::TitleStroke==6);
  {
    dashboard::Canvas title(frame.data(),font.data()); title.clear();
    title.text("A",64,24,184,92);
    const auto normal=frame;
    title.clear(); title.text("A",64,24,184,92,0,dashboard::TitleStroke);
    int added=0;
    for(int y=0;y<dashboard::Height;++y) for(int x=0;x<dashboard::Width;++x) {
      const auto before=at(normal,x,y),after=at(frame,x,y);
      assert(after<=before);
      if(after!=before) { ++added; assert(x>=64 && x<156 && y>=24 && y<208); }
    }
    assert(added>0);
  }
  for (const int size:{16,17,18,20,22,24,26,28,30,32,36,40,44,48,56,64,80,88,96,128,156,184}) {
    for (const bool ascii:{true,false}) {
      const int width=(ascii?8:16)*size/16, gw=ascii?8:16;
      dashboard::Canvas canvas(frame.data(),font.data()); canvas.clear();
      canvas.text(ascii?"A":"中",10,10,size,width,4);
      std::vector<bool> expected(size_t(width)*size,false);
      for(int y=0;y<size;++y) for(int x=0;x<width;++x) {
        const int row=y*16/size,col=x*gw/width;
        if ((row==4 && col==3) || (row==15 && col==gw-1)) {
          for(int dx=0;dx<=2 && x+dx<width;++dx)
            expected[size_t(y)*width+x+dx]=true;
        }
      }
      for(int y=0;y<dashboard::Height;++y) for(int x=0;x<dashboard::Width;++x) {
        const bool ink=x>=10 && x<10+width && y>=10 && y<10+size
          && expected[size_t(y-10)*width+x-10];
        assert(at(frame,x,y)==(ink?4:15));
      }
      // Ellipsis and clipping retain their original layout bounds.
      canvas.clear(); canvas.text("AAAA",10,10,size,size,0);
      assert(at(frame,10+size-1,10+size-1)==0);
      assert(at(frame,10+size,10+size-1)==15);
      // Exact-fit ASCII must not be replaced by an ellipsis (dates/clocks).
      canvas.clear(); canvas.text("AA",10,10,size,2*size/2,0);
      const int asciiWidth=8*size/16;
      assert(at(frame,10+asciiWidth-1,10+size-1)==0);
      assert(at(frame,10+2*asciiWidth-1,10+size-1)==0);
      canvas.clear(); canvas.text("中",dashboard::Width-size, dashboard::Height-size,size,size,0);
      assert(at(frame,dashboard::Width-1,dashboard::Height-1)==0);
    }
  }
  assert(dashboard::ContentWidth==2*dashboard::ColumnWidth);
  assert(dashboard::NoteAngle==0.0);
  assert(dashboard::BannerWidth==704 && dashboard::BannerHeight==176);
  dashboard::Canvas canvas(frame.data(),font.data());
  for(const char* id:{"sprint","cruise","snack","sleep"}) {
    auto asset=banner_assets::find(id);
    size_t count=0;
    for(size_t i=0;i<asset.size;++i) count+=(asset.data[i]>>4)+1;
    assert(count==size_t(dashboard::BannerWidth)*dashboard::BannerHeight);
    canvas.clear(); dashboard::banner(canvas,id);
    int p=0;
    for(size_t i=0;i<asset.size;++i) for(int n=0;n<(asset.data[i]>>4)+1;++n,++p)
      assert(at(frame,dashboard::BannerX+p%dashboard::BannerWidth,dashboard::BannerY+p/dashboard::BannerWidth)==(asset.data[i]&15));
    for(int y=0;y<dashboard::Height;++y) for(int x=0;x<dashboard::Width;++x)
      if(x<dashboard::BannerX || x>=dashboard::BannerX+dashboard::BannerWidth || y<dashboard::BannerY || y>=dashboard::BannerY+dashboard::BannerHeight) assert(at(frame,x,y)==15);
  }
  canvas.clear(); dashboard::resetIcon(canvas,896,600);
  int ink=0,edges=0;
  for(int y=0;y<dashboard::Height;++y) for(int x=0;x<dashboard::Width;++x) {
    uint8_t gray=at(frame,x,y);
    if(x<816||x>976||y<520||y>680) assert(gray==15);
    if(gray<15) ++ink;
    if(gray>0 && gray<15) ++edges;
    if((x-896)*(x-896)+(y-600)*(y-600)<44*44) assert(gray==15);
  }
  assert(ink>2500 && edges>100);
  // Ink-centering must ignore asymmetric left/top glyph-cell padding.
  for(const char* text:{"A","AA","AAA"}) {
    canvas.clear(); canvas.centeredText(text,896,600,40);
    int left=1404,right=0,top=1872,bottom=0;
    for(int y=540;y<660;++y) for(int x=810;x<982;++x) if(at(frame,x,y)<15) {
      left=std::min(left,x);right=std::max(right,x);top=std::min(top,y);bottom=std::max(bottom,y);
    }
    assert(abs(left+right-2*896)<=1 && abs(top+bottom-2*600)<=1);
  }
  // Full-width ink isolates layout padding from individual glyph bearings.
  auto full=font.data()+0x4E2D*33; full[0]=16;
  for(int i=1;i<33;++i) full[i]=255;
  std::string filled;
  for(int i=0;i<28;++i) filled+="中";
  const int centers[][2]={{367,986},{1037,986},{367,1514},{1037,1514}};
  for(const auto& center:centers) {
    canvas.clear(); dashboard::note(canvas,filled.c_str(),center[0],center[1],dashboard::NoteAngle);
    int paperLeft=center[0]-584/2,paperTop=center[1]-452/2;
    for(int row=0;row<4;++row) {
      int y=paperTop+80+row*84+32;
      int left=paperLeft+3,right=paperLeft+580;
      while(left<=right && at(frame,left,y)==15) ++left;
      while(right>=left && at(frame,right,y)==15) --right;
      assert(left-paperLeft==68 && paperLeft+583-right==68);
      assert(right-left+1==7*64);
    }
  }
  for(const char* text:{"", "中\nAAAA\n中\nAAAA\n中", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}) {
    canvas.clear(); dashboard::note(canvas,text,367,1017,-2.5);
    for(int y=0;y<dashboard::Height;++y) for(int x=0;x<dashboard::Width;++x)
      if(y<739||y>1295||x<49||x>685) assert(at(frame,x,y)==15);
  }
  std::cout<<"Native text: two-pixel stroke, grayscale, all font sizes and clipping passed.\n";
}
