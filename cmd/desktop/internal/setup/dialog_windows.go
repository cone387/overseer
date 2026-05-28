//go:build windows

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ShowDialog displays a Windows GUI dialog for first-time setup using PowerShell WPF.
// Returns the user's input or an error if cancelled.
func ShowDialog() (*Result, error) {
	hostname, _ := os.Hostname()
	defaultName := "Desktop " + hostname

	// PowerShell WPF dialog for setup
	ps := fmt.Sprintf(`
Add-Type -AssemblyName PresentationFramework
Add-Type -AssemblyName PresentationCore

[xml]$xaml = @"
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="Overseer Desktop - 首次配置" Height="320" Width="450"
        WindowStartupLocation="CenterScreen" ResizeMode="NoResize">
    <Grid Margin="20">
        <Grid.RowDefinitions>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="15"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="15"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="15"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="*"/>
            <RowDefinition Height="Auto"/>
        </Grid.RowDefinitions>

        <TextBlock Grid.Row="0" FontSize="16" FontWeight="Bold" Text="欢迎使用 Overseer Desktop"/>

        <TextBlock Grid.Row="2" Text="服务器地址" FontWeight="SemiBold"/>
        <TextBox Grid.Row="3" Name="ServerURL" Text="http://localhost:9721" Margin="0,5,0,0"/>

        <TextBlock Grid.Row="5" Text="注册令牌" FontWeight="SemiBold"/>
        <PasswordBox Grid.Row="6" Name="Token" Margin="0,5,0,0"/>

        <TextBlock Grid.Row="8" Text="设备名称（可选）" FontWeight="SemiBold"/>
        <TextBox Grid.Row="9" Name="DeviceName" Text="%s" Margin="0,5,0,0"/>

        <StackPanel Grid.Row="11" Orientation="Horizontal" HorizontalAlignment="Right">
            <Button Name="BtnCancel" Content="取消" Width="80" Margin="0,0,10,0"/>
            <Button Name="BtnConnect" Content="连接" Width="80" IsDefault="True"/>
        </StackPanel>
    </Grid>
</Window>
"@

$reader = (New-Object System.Xml.XmlNodeReader $xaml)
$window = [Windows.Markup.XamlReader]::Load($reader)

$serverURL = $window.FindName("ServerURL")
$token = $window.FindName("Token")
$deviceName = $window.FindName("DeviceName")
$btnConnect = $window.FindName("BtnConnect")
$btnCancel = $window.FindName("BtnCancel")

$script:result = ""

$btnConnect.Add_Click({
    if ($serverURL.Text -eq "" -or $token.Password -eq "") {
        [System.Windows.MessageBox]::Show("请填写服务器地址和注册令牌", "提示", "OK", "Warning")
        return
    }
    $script:result = "$($serverURL.Text)|$($token.Password)|$($deviceName.Text)"
    $window.Close()
})

$btnCancel.Add_Click({
    $script:result = ""
    $window.Close()
})

$window.ShowDialog() | Out-Null
Write-Output $script:result
`, strings.ReplaceAll(defaultName, `"`, `""`))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dialog error: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return nil, fmt.Errorf("setup cancelled by user")
	}

	parts := strings.SplitN(result, "|", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid dialog result")
	}

	name := defaultName
	if len(parts) >= 3 && parts[2] != "" {
		name = parts[2]
	}

	return &Result{
		ServerURL: parts[0],
		Token:     parts[1],
		Name:      name,
	}, nil
}
