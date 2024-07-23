#ifdef _WIN32
#include <Windows.h>
#include <WinNls.h>
#include <ShObjIdl.h>
#include <ShlGuid.h>
#endif

#include <memory>
#include <filesystem>
#include <string>
#include <codecvt>
#include <locale>


#ifdef _WIN32
extern "C"
HRESULT CreateLink(const wchar_t* lpszPathObj, const wchar_t* lpszPathLink)
{
    HRESULT hres;
    hres = CoInitializeEx(0, COINIT_MULTITHREADED | COINIT_SPEED_OVER_MEMORY);
    if (FAILED(hres)) {
        return hres;
    }
    std::shared_ptr<int> couninit(nullptr, [](int*){
        CoUninitialize();
    });

    IShellLink* psl;

    // Get a pointer to the IShellLink interface. It is assumed that CoInitialize
    // has already been called.
    hres = CoCreateInstance(CLSID_ShellLink, NULL, CLSCTX_INPROC_SERVER, IID_IShellLink, (LPVOID*)&psl);
    if (SUCCEEDED(hres))
    {
        IPersistFile* ppf;

        // Set the path to the shortcut target and add the description. 
        psl->SetPath(lpszPathObj);
        //psl->SetDescription(lpszDesc);

        // Query IShellLink for the IPersistFile interface, used for saving the 
        // shortcut in persistent storage. 
        hres = psl->QueryInterface(IID_IPersistFile, (LPVOID*)&ppf);

        if (SUCCEEDED(hres))
        {
            //WCHAR wsz[MAX_PATH];

            // Ensure that the string is Unicode. 
           // MultiByteToWideChar(CP_ACP, 0, lpszPathLink, -1, wsz, MAX_PATH);

            // Add code here to check return value from MultiByteWideChar 
            // for success.

            // Save the link by calling IPersistFile::Save. 
            hres = ppf->Save(lpszPathLink, TRUE);
            ppf->Release();
        }
        psl->Release();
    }
    return hres;
}
#endif

std::wstring utf8_to_utf16(const char* str) {
    std::wstring_convert<std::codecvt_utf8_utf16<wchar_t>> cvt;
    return cvt.from_bytes(str);
}

extern "C" 
int CreateSymLink(const char* path, const char* sympath) {
#if defined _WIN32
    auto wpath = utf8_to_utf16(path);
    auto wsympath = utf8_to_utf16(sympath);
    return CreateLink(wpath.c_str(), wsympath.c_str());
#elif defined __unix__
    std::error_code ec;
    std::filesystem::create_symlink(path, sympath, ec);
    return ec.value();
#endif
}

// int main() {
//     return 0;
// }