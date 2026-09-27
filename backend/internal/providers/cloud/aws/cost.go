package aws

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	costtypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

func (c *Client) Costs(ctx context.Context, start, end time.Time) (CostReport, error) {
	if !start.Before(end) {
		return CostReport{}, errors.New("cost period start must be before end")
	}
	period := &costtypes.DateInterval{Start: sdk.String(start.Format("2006-01-02")), End: sdk.String(end.Format("2006-01-02"))}
	daily, err := c.cost.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{TimePeriod: period, Granularity: costtypes.GranularityDaily, Metrics: []string{"UnblendedCost"}})
	if err != nil {
		return CostReport{}, safeError("read AWS Cost Explorer", err)
	}
	services, err := c.cost.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{TimePeriod: period, Granularity: costtypes.GranularityMonthly, Metrics: []string{"UnblendedCost"}, GroupBy: []costtypes.GroupDefinition{{Key: sdk.String("SERVICE"), Type: costtypes.GroupDefinitionTypeDimension}}})
	if err != nil {
		return CostReport{}, safeError("read AWS service costs", err)
	}
	regions, err := c.cost.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{TimePeriod: period, Granularity: costtypes.GranularityMonthly, Metrics: []string{"UnblendedCost"}, GroupBy: []costtypes.GroupDefinition{{Key: sdk.String("REGION"), Type: costtypes.GroupDefinitionTypeDimension}}})
	if err != nil {
		return CostReport{}, safeError("read AWS regional costs", err)
	}
	report := CostReport{PeriodStart: start, PeriodEnd: end, Unit: "USD", Freshness: "AWS Cost Explorer data is delayed and is not real-time."}
	for _, item := range daily.ResultsByTime {
		amount, unit := metric(item.Total, "UnblendedCost")
		pointStart, _ := time.Parse("2006-01-02", sdk.ToString(item.TimePeriod.Start))
		pointEnd, _ := time.Parse("2006-01-02", sdk.ToString(item.TimePeriod.End))
		report.Daily = append(report.Daily, CostPoint{Start: pointStart, End: pointEnd, Amount: amount, Unit: unit})
		report.Total += amount
	}
	report.Services = normalizeGroups(services.ResultsByTime)
	report.Regions = normalizeGroups(regions.ResultsByTime)
	if projects, tagErr := c.tagCosts(ctx, period, "silicon:project"); tagErr == nil {
		report.Projects = projects
		report.ProjectAttributionAvailable = true
	}
	if environments, tagErr := c.tagCosts(ctx, period, "silicon:environment"); tagErr == nil {
		report.Environments = environments
		report.EnvironmentAttributionAvailable = true
	}
	previousStart := start.AddDate(0, -1, 0)
	previous, previousErr := c.cost.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{TimePeriod: &costtypes.DateInterval{Start: sdk.String(previousStart.Format("2006-01-02")), End: sdk.String(start.Format("2006-01-02"))}, Granularity: costtypes.GranularityMonthly, Metrics: []string{"UnblendedCost"}})
	if previousErr == nil {
		value := 0.0
		for _, item := range previous.ResultsByTime {
			amount, _ := metric(item.Total, "UnblendedCost")
			value += amount
		}
		value = roundCurrency(value)
		report.PreviousPeriodTotal = &value
	}
	now := time.Now().UTC()
	forecastStart := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	forecastEnd := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	if forecastStart.Before(forecastEnd) {
		forecast, forecastErr := c.cost.GetCostForecast(ctx, &costexplorer.GetCostForecastInput{TimePeriod: &costtypes.DateInterval{Start: sdk.String(forecastStart.Format("2006-01-02")), End: sdk.String(forecastEnd.Format("2006-01-02"))}, Granularity: costtypes.GranularityMonthly, Metric: costtypes.MetricUnblendedCost})
		if forecastErr == nil && forecast.Total != nil {
			value, _ := strconv.ParseFloat(sdk.ToString(forecast.Total.Amount), 64)
			report.Forecast = &value
		}
	}
	report.Total = roundCurrency(report.Total)
	return report, nil
}

func (c *Client) Estimate(ctx context.Context, input CreateMachineInput) (Estimate, error) {
	if input.InstanceType == "" || input.Region == "" {
		return Estimate{}, errors.New("instance type and region are required for an estimate")
	}
	hourly, err := c.price(ctx, "AmazonEC2", []pricingtypes.Filter{{Field: sdk.String("instanceType"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String(input.InstanceType)}, {Field: sdk.String("regionCode"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String(input.Region)}, {Field: sdk.String("operatingSystem"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String("Linux")}, {Field: sdk.String("tenancy"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String("Shared")}, {Field: sdk.String("preInstalledSw"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String("NA")}, {Field: sdk.String("capacitystatus"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String("Used")}}, "Hrs")
	if err != nil {
		return Estimate{}, err
	}
	storageGiB := input.RootDiskGiB
	if storageGiB < 8 {
		storageGiB = 20
	}
	storageType := input.RootDiskType
	if storageType == "" {
		storageType = "gp3"
	}
	perGiB, storageErr := c.price(ctx, "AmazonEC2", []pricingtypes.Filter{{Field: sdk.String("regionCode"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String(input.Region)}, {Field: sdk.String("productFamily"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String("Storage")}, {Field: sdk.String("volumeApiName"), Type: pricingtypes.FilterTypeTermMatch, Value: sdk.String(storageType)}}, "GB-Mo")
	if storageErr != nil {
		return Estimate{}, storageErr
	}
	compute := roundCurrency(hourly * 730)
	storage := roundCurrency(perGiB * float64(storageGiB))
	return Estimate{ComputeMonthly: compute, StorageMonthly: storage, TotalMonthly: roundCurrency(compute + storage), Currency: "USD", Source: "AWS public on-demand pricing", Exclusions: []string{"Data transfer, public IPv4, Elastic IP, snapshot, IOPS, throughput, and tax charges are not included.", "Actual usage and regional pricing changes may differ."}}, nil
}

func (c *Client) price(ctx context.Context, service string, filters []pricingtypes.Filter, unit string) (float64, error) {
	out, err := c.pricing.GetProducts(ctx, &pricing.GetProductsInput{ServiceCode: sdk.String(service), Filters: filters, MaxResults: sdk.Int32(100)})
	if err != nil {
		return 0, safeError("read AWS public pricing", err)
	}
	lowest := math.MaxFloat64
	for _, document := range out.PriceList {
		var product struct {
			Terms map[string]map[string]struct {
				PriceDimensions map[string]struct {
					Unit         string            `json:"unit"`
					PricePerUnit map[string]string `json:"pricePerUnit"`
				} `json:"priceDimensions"`
			} `json:"terms"`
		}
		if json.Unmarshal([]byte(document), &product) != nil {
			continue
		}
		for _, term := range product.Terms["OnDemand"] {
			for _, dimension := range term.PriceDimensions {
				if dimension.Unit != unit {
					continue
				}
				value, _ := strconv.ParseFloat(dimension.PricePerUnit["USD"], 64)
				if value > 0 && value < lowest {
					lowest = value
				}
			}
		}
	}
	if lowest == math.MaxFloat64 {
		return 0, errors.New("AWS pricing was unavailable for the selected configuration")
	}
	return lowest, nil
}
func metric(values map[string]costtypes.MetricValue, key string) (float64, string) {
	value, ok := values[key]
	if !ok {
		return 0, "USD"
	}
	amount, _ := strconv.ParseFloat(sdk.ToString(value.Amount), 64)
	unit := sdk.ToString(value.Unit)
	if unit == "" {
		unit = "USD"
	}
	return amount, unit
}
func normalizeGroups(periods []costtypes.ResultByTime) []CostGroup {
	result := []CostGroup{}
	for _, period := range periods {
		for _, group := range period.Groups {
			name := strings.Join(group.Keys, " / ")
			amount, unit := metric(group.Metrics, "UnblendedCost")
			result = append(result, CostGroup{Name: name, Amount: roundCurrency(amount), Unit: unit})
		}
	}
	return result
}
func (c *Client) tagCosts(ctx context.Context, period *costtypes.DateInterval, tag string) ([]CostGroup, error) {
	result, err := c.cost.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{TimePeriod: period, Granularity: costtypes.GranularityMonthly, Metrics: []string{"UnblendedCost"}, GroupBy: []costtypes.GroupDefinition{{Key: sdk.String(tag), Type: costtypes.GroupDefinitionTypeTag}}})
	if err != nil {
		return nil, err
	}
	return normalizeTagGroups(tag, result.ResultsByTime), nil
}
func normalizeTagGroups(tag string, periods []costtypes.ResultByTime) []CostGroup {
	groups := normalizeGroups(periods)
	prefix := tag + "$"
	for index := range groups {
		groups[index].Name = strings.TrimPrefix(groups[index].Name, prefix)
		if groups[index].Name == "" {
			groups[index].Name = "Unallocated"
		}
	}
	return groups
}
func roundCurrency(value float64) float64 { return math.Round(value*100) / 100 }
