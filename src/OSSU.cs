using System;
using System.IO;
using System.Drawing;
using System.Drawing.Imaging;
using System.Windows.Forms;
using System.Runtime.InteropServices;
using System.Xml.Serialization;
using System.Threading;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Linq;

namespace OSSU {
public class Settings {
 public string Folder = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.MyPictures), "OSSU");
 public string Prefix = "Screenshot";
 public string Format = "PNG";
 public int Quality = 90;
 public bool Sequential = false;
 public int IntervalSeconds = 60;
 public string Target = "Active window";
 public bool CropTitlebar = true;
 public int WindowKey = (int)Keys.F9;
 public int DesktopKey = (int)Keys.F10;
 public static string Location { get { return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "OSSU", "settings.xml"); } }
 public static Settings Load(string path) {
  if (!File.Exists(path)) return new Settings();
  using(var s=File.OpenRead(path)) return (Settings)new XmlSerializer(typeof(Settings)).Deserialize(s);
 }
 public void Save(string path) {
  Directory.CreateDirectory(Path.GetDirectoryName(path)); string tmp=path+".tmp";
  using(var s=new FileStream(tmp,FileMode.Create,FileAccess.Write)) new XmlSerializer(typeof(Settings)).Serialize(s,this);
  if(File.Exists(path)) File.Replace(tmp,path,null); else File.Move(tmp,path);
 }
 public void Validate() {
  if(String.IsNullOrWhiteSpace(Folder) || !Path.IsPathRooted(Folder)) throw new ArgumentException("Choose an absolute output folder.");
  if(String.IsNullOrWhiteSpace(Prefix) || Prefix.IndexOfAny(Path.GetInvalidFileNameChars())>=0 || Prefix.EndsWith(".") || Prefix.EndsWith(" ")) throw new ArgumentException("Use a filename prefix without reserved characters or trailing spaces/dots.");
  if(Format!="PNG" && Format!="JPEG" && Format!="BMP") throw new ArgumentException("Choose PNG, JPEG or BMP.");
  if(Quality<1 || Quality>100 || IntervalSeconds<5 || IntervalSeconds>3600) throw new ArgumentException("Invalid quality or shutter interval.");
  if(WindowKey==DesktopKey) throw new ArgumentException("The two hotkeys must differ.");
 }
}
public static class Images {
 public static string Save(Bitmap image,Settings s,DateTime time) {
  s.Validate(); Directory.CreateDirectory(s.Folder);
  string ext=s.Format=="JPEG"?"jpg":s.Format.ToLowerInvariant();
  string stem=s.Prefix+(s.Sequential?"_": "_"+time.ToString("yyyy-MM-dd_HH-mm-ss-fff")+"_");
  for(int i=1;i<int.MaxValue;i++) {
   string path=Path.Combine(s.Folder,stem+i.ToString("D4")+"."+ext);
   FileStream stream;
   try { stream=new FileStream(path,FileMode.CreateNew,FileAccess.Write,FileShare.None); }
   catch(IOException) { if(File.Exists(path)) continue; throw; }
   try {
    using(stream) {
     if(s.Format=="JPEG") { var enc=ImageCodecInfo.GetImageEncoders().First(e=>e.FormatID==ImageFormat.Jpeg.Guid); using(var p=new EncoderParameters(1)) { p.Param[0]=new EncoderParameter(System.Drawing.Imaging.Encoder.Quality,(long)s.Quality); image.Save(stream,enc,p); } }
     else image.Save(stream,s.Format=="PNG"?ImageFormat.Png:ImageFormat.Bmp);
    }
    return path;
   } catch { try { File.Delete(path); } catch {} throw; }
  }
  throw new IOException("No available filename.");
 }
}
public static class Native {
 [DllImport("user32.dll")] public static extern bool RegisterHotKey(IntPtr h,int id,uint mods,uint key);
 [DllImport("user32.dll")] public static extern bool UnregisterHotKey(IntPtr h,int id);
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h,out Rect r);
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h,out Rect r);
 [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h,ref Point p);
 [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr h);
 [DllImport("user32.dll")] public static extern bool SetProcessDpiAwarenessContext(IntPtr value);
 [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
 [StructLayout(LayoutKind.Sequential)] public struct Rect {public int Left,Top,Right,Bottom;}
 public static Rectangle Bounds(string mode,bool client,IntPtr window) {
  if(mode=="All monitors") return SystemInformation.VirtualScreen;
  if(mode=="Current monitor") return Screen.FromHandle(window).Bounds;
  if(window==IntPtr.Zero || IsIconic(window)) throw new InvalidOperationException("No visible active window to capture.");
  Rect r;
  if(client) { if(!GetClientRect(window,out r)) throw new InvalidOperationException("Cannot read client area."); Point p=Point.Empty; if(!ClientToScreen(window,ref p)) throw new InvalidOperationException("Cannot locate client area."); return Rectangle.Intersect(new Rectangle(p.X,p.Y,r.Right-r.Left,r.Bottom-r.Top),SystemInformation.VirtualScreen); }
  if(!GetWindowRect(window,out r)) throw new InvalidOperationException("Cannot read window bounds.");
  return Rectangle.Intersect(Rectangle.FromLTRB(r.Left,r.Top,r.Right,r.Bottom),SystemInformation.VirtualScreen);
 }
 public static Bitmap Capture(Rectangle r) {
  if(r.Width<=0 || r.Height<=0 || (long)r.Width*r.Height>100000000) throw new InvalidOperationException("Capture area is empty or too large.");
  Bitmap b=new Bitmap(r.Width,r.Height,PixelFormat.Format24bppRgb);
  try { using(Graphics g=Graphics.FromImage(b)) g.CopyFromScreen(r.Location,Point.Empty,r.Size,CopyPixelOperation.SourceCopy); return b; } catch {b.Dispose(); throw;}
 }
}
public class MainForm : Form {
 Settings settings; TextBox folder=new TextBox(),prefix=new TextBox(); ComboBox format=new ComboBox(),target=new ComboBox(),windowKey=new ComboBox(),desktopKey=new ComboBox();
 NumericUpDown quality=new NumericUpDown(),interval=new NumericUpDown(); CheckBox sequence=new CheckBox(),crop=new CheckBox();
 Label status=new Label(); Button auto=new Button(); PictureBox preview=new PictureBox(); ListBox recent=new ListBox();
 NotifyIcon tray; System.Windows.Forms.Timer timer=new System.Windows.Forms.Timer(); bool busy,closing,registered; string configPath;
 List<string> files=new List<string>();
 public MainForm(bool test) {
  configPath=test?null:Settings.Location;
  string loadError=null; try {settings=test?new Settings():Settings.Load(configPath);} catch(Exception e) {settings=new Settings(); loadError="Settings unreadable; defaults loaded: "+e.Message;}
  Text="OSSU - Open Source Screenshot Utility"; ClientSize=new Size(680,640); MinimumSize=new Size(650,660); Font=new Font("Segoe UI",9); AutoScaleMode=AutoScaleMode.None;
  var layout=new TableLayoutPanel{Dock=DockStyle.Fill,Padding=new Padding(16),ColumnCount=3,RowCount=12};
  layout.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute,140)); layout.ColumnStyles.Add(new ColumnStyle(SizeType.Percent,100));layout.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute,110));Controls.Add(layout);
  Action<int,string,Control> row=(n,label,c)=>{layout.Controls.Add(new Label{Text=label,AutoSize=true,Anchor=AnchorStyles.Left},0,n);c.Dock=DockStyle.Fill;layout.Controls.Add(c,1,n);};
  row(0,"Save screenshots to",folder);folder.Text=settings.Folder;var browse=new Button{Text="Browse...",Dock=DockStyle.Fill};layout.Controls.Add(browse,2,0);browse.Click+=(o,e)=>{using(var d=new FolderBrowserDialog{SelectedPath=folder.Text})if(d.ShowDialog(this)==DialogResult.OK)folder.Text=d.SelectedPath;};
  row(1,"File name prefix",prefix);prefix.Text=settings.Prefix;sequence.Text="Sequence";sequence.Checked=settings.Sequential;layout.Controls.Add(sequence,2,1);
  row(2,"Image format",format);format.DropDownStyle=ComboBoxStyle.DropDownList;format.Items.AddRange(new object[]{"PNG","JPEG","BMP"});format.SelectedItem=settings.Format;quality.Minimum=1;quality.Maximum=100;quality.Value=Math.Max(1,Math.Min(100,settings.Quality));layout.Controls.Add(quality,2,2);format.SelectedIndexChanged+=(o,e)=>quality.Enabled=(string)format.SelectedItem=="JPEG";quality.Enabled=settings.Format=="JPEG";
  row(3,"Capture target",target);target.DropDownStyle=ComboBoxStyle.DropDownList;target.Items.AddRange(new object[]{"Active window","Current monitor","All monitors"});target.SelectedItem=settings.Target;crop.Text="Client area";crop.Checked=settings.CropTitlebar;layout.Controls.Add(crop,2,3);
  row(4,"Target hotkey",windowKey); row(5,"All-screen hotkey",desktopKey);
  foreach(var c in new[]{windowKey,desktopKey}) { c.DropDownStyle=ComboBoxStyle.DropDownList;for(int i=1;i<=12;i++)c.Items.Add("Ctrl+Shift+F"+i); }
  windowKey.SelectedIndex=Math.Max(0,Math.Min(11,settings.WindowKey-(int)Keys.F1));desktopKey.SelectedIndex=Math.Max(0,Math.Min(11,settings.DesktopKey-(int)Keys.F1));
  var apply=new Button{Text="Apply settings",Dock=DockStyle.Fill};layout.Controls.Add(apply,2,5);apply.Click+=(o,e)=>Apply();
  interval.Minimum=5;interval.Maximum=3600;interval.Value=Math.Max(5,Math.Min(3600,settings.IntervalSeconds));row(6,"Auto shutter (sec)",interval);auto.Text="Start auto";auto.Dock=DockStyle.Fill;layout.Controls.Add(auto,2,6);auto.Click+=(o,e)=>ToggleAuto();
  var buttons=new FlowLayoutPanel{Dock=DockStyle.Fill,AutoSize=true};layout.Controls.Add(buttons,0,7);layout.SetColumnSpan(buttons,3);
  var capture=new Button{Text="Capture in 3s",AutoSize=true};buttons.Controls.Add(capture);capture.Click+=async(o,e)=>{if(!Apply())return;capture.Enabled=false;status.Text="Switch to the desired window. Capturing in 3 seconds...";await Task.Delay(3000);if(!IsDisposed)await Take(settings.Target);if(!IsDisposed)capture.Enabled=true;};
  var open=new Button{Text="Open folder",AutoSize=true};buttons.Controls.Add(open);open.Click+=(o,e)=>{try {if(!Apply())return;Directory.CreateDirectory(settings.Folder);System.Diagnostics.Process.Start("explorer.exe", "\""+settings.Folder+"\"");}catch(Exception ex){status.Text=ex.Message;}};
  var hide=new Button{Text="Hide to tray",AutoSize=true};buttons.Controls.Add(hide);hide.Click+=(o,e)=>Hide();
  var note=new Label{Text="Hotkeys capture visible pixels without activating the game. Auto shutter is off at startup.\nPNG is lossless. JPEG quality is 1-100. No upload, telemetry or game injection.",Dock=DockStyle.Fill,AutoSize=true};layout.Controls.Add(note,0,8);layout.SetColumnSpan(note,3);
  status.Text=loadError??"Ready. Apply settings to activate hotkeys.";status.Dock=DockStyle.Fill;status.AutoSize=true;layout.Controls.Add(status,0,9);layout.SetColumnSpan(status,3);
  preview.Dock=DockStyle.Fill;preview.SizeMode=PictureBoxSizeMode.Zoom;preview.BackColor=Color.FromArgb(35,38,42);layout.Controls.Add(preview,1,10);layout.SetColumnSpan(preview,2);recent.Dock=DockStyle.Fill;layout.Controls.Add(recent,0,10);layout.RowStyles.Clear();for(int i=0;i<10;i++)layout.RowStyles.Add(new RowStyle(SizeType.Absolute,i<7?32:(i==7?40:(i==8?52:44))));layout.RowStyles.Add(new RowStyle(SizeType.Percent,100));
  recent.SelectedIndexChanged+=(o,e)=>{if(recent.SelectedIndex<0)return;try{using(var img=Image.FromFile(files[recent.SelectedIndex])) {var old=preview.Image;preview.Image=new Bitmap(img);if(old!=null)old.Dispose();}}catch(Exception ex){status.Text=ex.Message;}};
  timer.Tick+=async(o,e)=>await Take(settings.Target);
  using(var g=CreateGraphics()){float scale=g.DpiX/96f;if(scale!=1f)Scale(new SizeF(scale,scale));}
  if(!test) {
   var menu=new ContextMenuStrip();menu.Items.Add("Show OSSU",null,(o,e)=>{Show();WindowState=FormWindowState.Normal;Activate();});menu.Items.Add("Stop auto shutter",null,(o,e)=>StopAuto());menu.Items.Add("Exit",null,(o,e)=>{closing=true;Close();});
   tray=new NotifyIcon{Icon=SystemIcons.Application,Text="OSSU - ready",Visible=true,ContextMenuStrip=menu};tray.DoubleClick+=(o,e)=>{Show();Activate();};Shown+=(o,e)=>Apply();
   FormClosing+=(o,e)=>{if(!closing && e.CloseReason==CloseReason.UserClosing){e.Cancel=true;Hide();status.Text="Hidden to tray. Exit from tray menu.";}};
  }
 }
 Settings ReadSettings() {var s=new Settings{Folder=folder.Text,Prefix=prefix.Text,Format=(string)format.SelectedItem,Quality=(int)quality.Value,Sequential=sequence.Checked,IntervalSeconds=(int)interval.Value,Target=(string)target.SelectedItem,CropTitlebar=crop.Checked,WindowKey=(int)Keys.F1+windowKey.SelectedIndex,DesktopKey=(int)Keys.F1+desktopKey.SelectedIndex};s.Validate();return s;}
 bool Apply() {
  try {var s=ReadSettings();Native.UnregisterHotKey(Handle,1);Native.UnregisterHotKey(Handle,2);registered=false;
   bool a=Native.RegisterHotKey(Handle,1,0x4006,(uint)s.WindowKey);bool b=Native.RegisterHotKey(Handle,2,0x4006,(uint)s.DesktopKey);
   if(!a||!b) {Native.UnregisterHotKey(Handle,1);Native.UnregisterHotKey(Handle,2);StopAuto();status.Text="Hotkey unavailable. Choose different keys and Apply; neither hotkey is active.";return false;}
   registered=true;settings=s;if(configPath!=null)s.Save(configPath);timer.Interval=s.IntervalSeconds*1000;status.Text="Settings saved. Hotkeys active.";return true;
  }catch(Exception e){status.Text=e.Message;return false;}
 }
 void ToggleAuto(){if(timer.Enabled){StopAuto();return;}if(!Apply())return;timer.Start();auto.Text="Stop auto";status.Text="AUTO SHUTTER ON: captures selected target until stopped.";if(tray!=null)tray.Text="OSSU - AUTO SHUTTER ON";}
 void StopAuto(){timer.Stop();auto.Text="Start auto";if(tray!=null)tray.Text="OSSU - ready";}
 async Task Take(string mode) {
  if(busy)return;busy=true;
  try {IntPtr foreground=Native.GetForegroundWindow();if(foreground==Handle && mode=="Active window")throw new InvalidOperationException("Switch to your game, then use the hotkey or delayed capture.");Rectangle bounds=Native.Bounds(mode,settings.CropTitlebar,foreground);Settings snapshot=settings;
   string path=await Task.Run(()=>{using(var bitmap=Native.Capture(bounds))return Images.Save(bitmap,snapshot,DateTime.Now);});
   if(IsDisposed)return;files.Insert(0,path);recent.Items.Insert(0,Path.GetFileName(path));if(files.Count>30){files.RemoveAt(30);recent.Items.RemoveAt(30);}recent.SelectedIndex=0;status.Text="Saved "+Path.GetFileName(path);if(tray!=null)tray.Text=timer.Enabled?"OSSU - AUTO SHUTTER ON":"OSSU - captured";
  }catch(Exception e){if(!IsDisposed){status.Text="Capture failed: "+e.Message;StopAuto();}}finally{busy=false;}
 }
 protected override void WndProc(ref Message m){if(m.Msg==0x312 && registered){var unused=Take(m.WParam.ToInt32()==2?"All monitors":settings.Target);}base.WndProc(ref m);}
 protected override void Dispose(bool disposing){if(disposing){timer.Dispose();if(IsHandleCreated){Native.UnregisterHotKey(Handle,1);Native.UnregisterHotKey(Handle,2);}if(tray!=null){tray.Visible=false;tray.Dispose();}if(preview.Image!=null)preview.Image.Dispose();}base.Dispose(disposing);}
}
static class Program {
 [STAThread] static int Main(string[] args) {
  try{Native.SetProcessDpiAwarenessContext(new IntPtr(-4));}catch{Native.SetProcessDPIAware();}
  Application.EnableVisualStyles();Application.SetCompatibleTextRenderingDefault(false);
  if(args.Length>0 && args[0]=="--self-test")return Tests.Run(args.Length>1?args[1]:Path.Combine(Path.GetTempPath(),"ossu-test-"+Guid.NewGuid().ToString("N")));
  bool fresh;using(var mutex=new Mutex(true,"Local\\OSSU.ScreenCapture",out fresh)){if(!fresh){MessageBox.Show("OSSU is already running. Open it from the system tray.");return 0;}Application.Run(new MainForm(false));}return 0;
 }
}
static class Tests {
 public static int Run(string dir) {
  Directory.CreateDirectory(dir);var results=new List<string>();Action<bool,string> check=(b,n)=>{if(!b)throw new Exception(n);results.Add("PASS "+n);};
  try {
   var s=new Settings{Folder=Path.Combine(dir,"captures"),Prefix="Test"};s.Validate();check(true,"default settings valid");
   using(var bitmap=new Bitmap(320,180)){using(var g=Graphics.FromImage(bitmap)){g.Clear(Color.CornflowerBlue);g.DrawString("OSSU synthetic capture test",SystemFonts.DefaultFont,Brushes.White,20,20);}foreach(string f in new[]{"PNG","JPEG","BMP"}){s.Format=f;string path=Images.Save(bitmap,s,new DateTime(2026,1,1));using(var img=Image.FromFile(path))check(img.Width==320&&img.Height==180,f+" encode and decode");}
    s.Format="PNG";s.Sequential=true;string one=Images.Save(bitmap,s,DateTime.Now),two=Images.Save(bitmap,s,DateTime.Now);check(one!=two,"sequence never overwrites");s.Sequential=false;one=Images.Save(bitmap,s,new DateTime(2026,1,1));two=Images.Save(bitmap,s,new DateTime(2026,1,1));check(one!=two,"timestamp collision never overwrites");}
   string xml=Path.Combine(dir,"settings.xml");s.Save(xml);check(Settings.Load(xml).Prefix=="Test","settings round trip");s.Prefix="Updated";s.Save(xml);check(Settings.Load(xml).Prefix=="Updated","atomic settings replacement");
   foreach(string bad in new[]{"..\\escape","bad:name","bad.",""}){s.Prefix=bad;bool rejected=false;try{s.Validate();}catch(ArgumentException){rejected=true;}check(rejected,"reject invalid prefix "+bad);}
   using(var form=new MainForm(true)){form.StartPosition=FormStartPosition.Manual;form.Location=new Point(-30000,-30000);form.ShowInTaskbar=false;form.Show();Application.DoEvents();form.PerformLayout();using(var image=new Bitmap(form.Width,form.Height)){form.DrawToBitmap(image,new Rectangle(Point.Empty,form.Size));image.Save(Path.Combine(dir,"ossu-ui-preview.png"),ImageFormat.Png);}check(form.Controls.Count>0,"actual WinForms layout renders without desktop capture");}
   File.WriteAllLines(Path.Combine(dir,"test-results.txt"),results.Concat(new[]{"ALL "+results.Count+" CHECKS PASSED"}));return 0;
  }catch(Exception e){results.Add("FAIL "+e);File.WriteAllLines(Path.Combine(dir,"test-results.txt"),results);return 1;}
 }
}
}
