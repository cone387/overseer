//go:build windows

package setup

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/locale"
)

// ShowSettingsDialog displays a settings dialog on Windows.
func ShowSettingsDialog(cfg *config.Config) (newServerURL string, reRegister bool, err error) {
	ps := fmt.Sprintf(`
Add-Type -AssemblyName PresentationFramework

[xml]$xaml = @"
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="%s" Height="300" Width="440"
        WindowStartupLocation="CenterScreen" ResizeMode="NoResize"
        Background="#F9FAFB">
    <Grid Margin="24">
        <Grid.RowDefinitions>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="14"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="14"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="Auto"/>
            <RowDefinition Height="*"/>
            <RowDefinition Height="Auto"/>
        </Grid.RowDefinitions>

        <TextBlock Grid.Row="0" FontSize="12" Foreground="#6B7280" FontWeight="SemiBold" Text="%s" Margin="0,0,0,4"/>
        <TextBox Grid.Row="1" Name="ServerURL" Text="%s" Padding="8,6" FontSize="13" BorderBrush="#D1D5DB"/>

        <TextBlock Grid.Row="3" FontSize="12" Foreground="#6B7280" FontWeight="SemiBold" Text="%s" Margin="0,0,0,4"/>
        <TextBlock Grid.Row="4" Text="%s" Padding="8,6" FontSize="13" Background="#F3F4F6" Foreground="#374151"/>

        <TextBlock Grid.Row="6" FontSize="12" Foreground="#6B7280" FontWeight="SemiBold" Text="%s" Margin="0,0,0,4"/>
        <TextBlock Grid.Row="7" Text="%s" Padding="8,6" FontSize="11" Background="#F3F4F6" Foreground="#6B7280" FontFamily="Consolas"/>

        <StackPanel Grid.Row="9" Orientation="Horizontal" HorizontalAlignment="Right" Margin="0,12,0,0">
            <Button Name="BtnReRegister" Content="%s" Padding="14,6" FontSize="12" Background="#EF4444" Foreground="White" BorderThickness="0" Margin="0,0,8,0" Cursor="Hand"/>
            <Button Name="BtnCancel" Content="%s" Padding="14,6" FontSize="12" Background="White" Foreground="#374151" BorderBrush="#D1D5DB" BorderThickness="1" Margin="0,0,8,0" Cursor="Hand"/>
            <Button Name="BtnSave" Content="%s" Padding="14,6" FontSize="12" Background="#4F46E5" Foreground="White" BorderThickness="0" Cursor="Hand" IsDefault="True"/>
        </StackPanel>
    </Grid>
</Window>
"@

$reader = (New-Object System.Xml.XmlNodeReader $xaml)
$window = [Windows.Markup.XamlReader]::Load($reader)

$serverURL = $window.FindName("ServerURL")
$btnSave = $window.FindName("BtnSave")
$btnCancel = $window.FindName("BtnCancel")
$btnReRegister = $window.FindName("BtnReRegister")

$script:result = ""

$btnSave.Add_Click({
    $script:result = "save|$($serverURL.Text)"
    $window.Close()
})

$btnCancel.Add_Click({
    $script:result = ""
    $window.Close()
})

$btnReRegister.Add_Click({
    $script:result = "reregister|$($serverURL.Text)"
    $window.Close()
})

$window.ShowDialog() | Out-Null
Write-Output $script:result
`,
		locale.T("settings.title"),
		locale.T("settings.server_url"),
		cfg.ServerURL,
		locale.T("settings.device_name"),
		cfg.DeviceName,
		locale.T("settings.device_id"),
		cfg.DeviceID,
		locale.T("settings.re_register"),
		locale.T("settings.cancel"),
		locale.T("settings.save"),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps)
	cmd.SysProcAttr = hiddenProcAttr()
	output, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("settings dialog error: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", false, nil
	}

	parts := strings.SplitN(result, "|", 2)
	action := parts[0]
	url := ""
	if len(parts) > 1 {
		url = parts[1]
	}

	switch action {
	case "save":
		return url, false, nil
	case "reregister":
		return url, true, nil
	default:
		return "", false, nil
	}
}
